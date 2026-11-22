// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Package native discovers, verifies, loads and binds the lancedb-c shared
// library without cgo.
//
// Discovery order:
//
//  1. LANCEDB_C_LIBRARY_PATH, which must name an existing regular file. When
//     set, it is the only candidate; a bad value fails instead of silently
//     falling back to another library.
//  2. A private per-user cache populated from the pinned lancedb-c release
//     archive. The archive and the extracted library are verified against the
//     published SHA-256 digests before the library is loaded.
//  3. The platform library name resolved by the system dynamic loader. This
//     supports package-manager and system installations, but still requires the
//     runtime version to match the header this module was built against.
//
// The library is loaded at most once per process and is never unloaded.
// lancedb-c owns a Tokio runtime, thread-local error state and handles that may
// outlive a table, so unloading it while native threads or callbacks may still
// run is not safe.
package native

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	// EnvLibraryPath names the explicit override. It must be an absolute or
	// relative path to the shared library file.
	EnvLibraryPath = "LANCEDB_C_LIBRARY_PATH"
	// EnvCacheDir overrides the root of the private native-library cache.
	EnvCacheDir = "LANCEDB_C_CACHE_DIR"
	// EnvReleaseBaseURL overrides the release download base URL. It is useful
	// for a mirror or a loopback fixture; the pinned archive names and digests
	// still apply.
	EnvReleaseBaseURL = "LANCEDB_C_RELEASE_BASE_URL"
)

// Source identifies how a library was selected.
type Source string

const (
	SourceEnvironment Source = "environment"
	SourceCache       Source = "cache"
	SourceSystem      Source = "system"
)

// Library is a loaded lancedb-c shared library.
type Library struct {
	Path   string
	Source Source
	Handle uintptr
}

// State is a side-effect-free snapshot of the loaded library.
type State struct {
	Loaded  bool
	Path    string
	Source  Source
	Version string
	Target  string
}

var (
	loadMu     sync.Mutex
	loadedAPI  *API
	loadedInfo State
)

// Load returns the process-wide bound C API, loading the shared library on
// first use. Failed attempts are not cached, so a later call retries discovery.
func Load() (*API, error) {
	loadMu.Lock()
	defer loadMu.Unlock()

	if loadedAPI != nil {
		return loadedAPI, nil
	}

	lib, err := discover()
	if err != nil {
		return nil, err
	}
	api, err := bindAPI(lib)
	if err != nil {
		return nil, err
	}
	if got := api.Version(); got != ReleaseVersion {
		return nil, fmt.Errorf("lancedb-c runtime version is %q, want %q; the Go binding is built against the header and ABI of lancedb-c %s", got, ReleaseVersion, ReleaseTag)
	}

	loadedAPI = api
	loadedInfo = State{
		Loaded:  true,
		Path:    lib.Path,
		Source:  lib.Source,
		Version: ReleaseVersion,
		Target:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	return api, nil
}

// Observe returns the current process state without loading a library.
func Observe() State {
	loadMu.Lock()
	defer loadMu.Unlock()
	return loadedInfo
}

func discover() (*Library, error) {
	if path := os.Getenv(EnvLibraryPath); path != "" {
		return openExplicit(path)
	}

	target, err := CurrentTarget()
	if err != nil {
		return nil, err
	}

	cacheErr := error(nil)
	path, err := install(target)
	if err == nil {
		lib, openErr := openLibraryPath(path, SourceCache)
		if openErr == nil {
			return lib, nil
		}
		// A cached file can become unusable after a partial OS update. Remove
		// our copy and retry the verified extraction once.
		if removeErr := os.Remove(path); removeErr == nil || os.IsNotExist(removeErr) {
			path, err = install(target)
			if err == nil {
				lib, openErr = openLibraryPath(path, SourceCache)
				if openErr == nil {
					return lib, nil
				}
			}
		} else {
			cacheErr = fmt.Errorf("cached library %s is unusable: %w", path, openErr)
		}
	}
	if err != nil && cacheErr == nil {
		cacheErr = err
	}

	lib, systemErr := openSystem(target)
	if systemErr == nil {
		return lib, nil
	}
	if cacheErr != nil {
		return nil, fmt.Errorf("lancedb-c %s for %s is unavailable: %v; system loader: %v", ReleaseTag, runtime.GOOS+"/"+runtime.GOARCH, cacheErr, systemErr)
	}
	return nil, systemErr
}

func openExplicit(path string) (*Library, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%s=%q: resolve absolute path: %w", EnvLibraryPath, path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("%s=%q: %w", EnvLibraryPath, path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s=%q is not a regular file (mode %s)", EnvLibraryPath, path, info.Mode())
	}
	return openLibraryPath(absolute, SourceEnvironment)
}

func openLibraryPath(path string, source Source) (*Library, error) {
	handle, err := openNativeLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("load lancedb-c library %s: %w", path, err)
	}
	return &Library{Path: path, Source: source, Handle: handle}, nil
}

func openSystem(target Target) (*Library, error) {
	handle, err := openNativeLibrary(target.LibraryName)
	if err != nil {
		return nil, fmt.Errorf("system loader could not open %q: %w; set %s to a released lancedb-c library or allow the private cache download", target.LibraryName, err, EnvLibraryPath)
	}
	return &Library{Path: target.LibraryName, Source: SourceSystem, Handle: handle}, nil
}

func cacheRoot() (string, error) {
	if root := os.Getenv(EnvCacheDir); root != "" {
		return root, nil
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate user cache directory: %w", err)
	}
	return root, nil
}

func cacheDirFor(target Target) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "lancedb-go", "native", ReleaseVersion, target.Name), nil
}

func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

var errDigestMismatch = errors.New("digest mismatch")

func verifyFile(path, want string) error {
	got, _, err := sha256File(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %s sha256 is %s, want %s", errDigestMismatch, path, got, want)
	}
	return nil
}

// resetForTest clears the cached handle for tests without unloading it.
func resetForTest() {
	loadMu.Lock()
	defer loadMu.Unlock()
	loadedAPI = nil
	loadedInfo = State{}
}
