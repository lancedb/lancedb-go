// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const downloadTimeout = 20 * time.Minute

// install returns the verified cache path for target, downloading and
// extracting the published archive when the cache does not already hold the
// exact library bytes.
func install(target Target) (string, error) {
	dir, err := cacheDirFor(target)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create native cache %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !isWindows() {
		return "", fmt.Errorf("restrict native cache %s: %w", dir, err)
	}
	lock, err := openCacheLock(dir)
	if err != nil {
		return "", err
	}
	defer lock.Close()

	libPath := filepath.Join(dir, target.LibraryName)
	usable, err := prepareCachedLibrary(libPath, target.LibrarySHA256)
	if err != nil {
		return "", err
	}
	if usable {
		// A verified library is a complete cache entry. Do not require the
		// archive or the network to reuse it, and do not replace its inode.
		return libPath, nil
	}

	archivePath := filepath.Join(dir, target.Archive)
	if err := verifyFileIfExists(archivePath, target.ArchiveSHA256); err != nil {
		return "", err
	}
	if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
		url := releaseBaseURL() + "/" + target.Archive
		if err := downloadFile(url, archivePath, target.ArchiveSHA256); err != nil {
			return "", fmt.Errorf("download %s: %w", url, err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, "."+target.LibraryName+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temporary library in %s: %w", dir, err)
	}
	tmp := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("close temporary library %s: %w", tmp, err)
	}
	defer os.Remove(tmp)
	if err := extractLibrary(archivePath, target, tmp); err != nil {
		return "", err
	}
	if err := verifyFile(tmp, target.LibrarySHA256); err != nil {
		return "", fmt.Errorf("extracted library: %w", err)
	}
	if err := os.Chmod(tmp, 0o500); err != nil && !isWindows() {
		return "", fmt.Errorf("protect extracted library: %w", err)
	}
	if err := os.Rename(tmp, libPath); err != nil {
		// Another installer may have published the same verified bytes before
		// this process acquired the shared cache lock.
		if verifyFile(libPath, target.LibrarySHA256) == nil {
			return libPath, nil
		}
		return "", fmt.Errorf("publish cached library %s: %w", libPath, err)
	}
	// Keep the verified archive. Removing it while another process is reading
	// the same cache entry creates a cross-process race; the archive is also
	// useful for an offline repair.
	return libPath, nil
}

func releaseBaseURL() string {
	if base := os.Getenv(EnvReleaseBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	return "https://github.com/" + ReleaseRepo + "/releases/download/" + ReleaseTag
}

func verifyFileIfExists(path, want string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if err := verifyFile(path, want); err != nil {
		if !errors.Is(err, errDigestMismatch) {
			// A transient open/read/sharing error is not a corrupt archive.
			return fmt.Errorf("verify archive %s: %w", path, err)
		}
		// A partial or stale download is not a usable cache entry. Remove it
		// and let the caller fetch the published archive again.
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove unusable archive %s: %w", path, removeErr)
		}
	}
	return nil
}

func downloadFile(url, destination, wantSHA256 string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary archive next to %s: %w", destination, err)
	}
	tmp := tmpFile.Name()
	defer os.Remove(tmp)
	f := tmpFile
	defer f.Close()

	client := &http.Client{Timeout: downloadTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lancedb-go/"+ReleaseVersion)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA256 {
		return fmt.Errorf("archive sha256 is %s, want %s", got, wantSHA256)
	}
	if err := os.Rename(tmp, destination); err != nil {
		if verifyFile(destination, wantSHA256) == nil {
			return nil
		}
		return fmt.Errorf("publish archive %s: %w", destination, err)
	}
	return nil
}

func extractLibrary(archivePath string, target Target, destination string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open archive %s: %w", archivePath, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive %s: %w", archivePath, err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(strings.TrimPrefix(hdr.Name, "./"))
		base := path.Base(name)
		// Linux stores the runtime bytes as liblancedb.so.0.1 behind the
		// liblancedb.so linker symlink. The other targets use the plain name.
		if base != target.LibraryName && base != target.LibraryName+".0.1" {
			continue
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, hdr.Size)); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("archive %s does not contain %s or %s.0.1", archivePath, target.LibraryName, target.LibraryName)
}

// prepareCachedLibrary verifies an existing library and returns true when it
// is already the exact target. It removes an entry only after a confirmed
// digest mismatch or non-regular file; transient stat/open/read errors are
// returned without modifying the cache.
func prepareCachedLibrary(path, wantSHA256 string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect cached library %s: %w", path, err)
	}
	if info.Mode().IsRegular() {
		verifyErr := verifyFile(path, wantSHA256)
		if verifyErr == nil {
			return true, nil
		}
		if !errors.Is(verifyErr, errDigestMismatch) {
			// A transient open/read/sharing error is not a corrupt library;
			// leave the entry in place and report the real failure.
			return false, fmt.Errorf("verify cached library %s: %w", path, verifyErr)
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("remove unusable cached library %s: %w", path, err)
	}
	return false, nil
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

// Install downloads, verifies and extracts the published artifact for the
// running target into the private cache. It returns the runtime library path.
// Load calls the same logic lazily; this entry point lets an installer or CI
// pre-populate the cache without loading the library.
func Install() (string, error) {
	target, err := CurrentTarget()
	if err != nil {
		return "", err
	}
	return install(target)
}
