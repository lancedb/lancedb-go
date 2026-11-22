// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestInstallDownloadsVerifiesAndReusesCache(t *testing.T) {
	library := []byte("fake runtime library bytes")
	archive := makeArchive(t, "lib/libfake.so", library)
	target := Target{
		Name:          "test-target",
		Archive:       "fake-runtime.tar.gz",
		ArchiveSHA256: sha256Hex(archive),
		LibraryPath:   "lib/libfake.so",
		LibraryName:   "libfake.so",
		LibrarySHA256: sha256Hex(library),
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/"+target.Archive {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	t.Setenv(EnvReleaseBaseURL, server.URL)
	t.Setenv(EnvCacheDir, t.TempDir())

	path, err := install(target)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed library: %v", err)
	}
	if !bytes.Equal(got, library) {
		t.Fatalf("installed bytes differ: %q", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("download requests = %d, want 1", requests.Load())
	}

	again, err := install(target)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if again != path {
		t.Fatalf("second install path = %q, want %q", again, path)
	}
	if requests.Load() != 1 {
		t.Fatalf("cache reuse downloaded again: %d requests", requests.Load())
	}
}

func TestInstallReplacesCorruptCache(t *testing.T) {
	library := []byte("correct runtime bytes")
	archive := makeArchive(t, "lib/libfake.so", library)
	target := Target{
		Name:          "test-target",
		Archive:       "fake-runtime.tar.gz",
		ArchiveSHA256: sha256Hex(archive),
		LibraryPath:   "lib/libfake.so",
		LibraryName:   "libfake.so",
		LibrarySHA256: sha256Hex(library),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	t.Setenv(EnvReleaseBaseURL, server.URL)
	t.Setenv(EnvCacheDir, t.TempDir())

	path, err := install(target)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("corrupt cache: %v", err)
	}
	replaced, err := install(target)
	if err != nil {
		t.Fatalf("replace corrupt cache: %v", err)
	}
	got, err := os.ReadFile(replaced)
	if err != nil {
		t.Fatalf("read replacement: %v", err)
	}
	if !bytes.Equal(got, library) {
		t.Fatalf("replacement bytes = %q", got)
	}
}

func TestInstallRejectsArchiveDigestMismatch(t *testing.T) {
	library := []byte("library")
	archive := makeArchive(t, "lib/libfake.so", library)
	target := Target{
		Name:          "test-target",
		Archive:       "fake-runtime.tar.gz",
		ArchiveSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		LibraryPath:   "lib/libfake.so",
		LibraryName:   "libfake.so",
		LibrarySHA256: sha256Hex(library),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	t.Setenv(EnvReleaseBaseURL, server.URL)
	t.Setenv(EnvCacheDir, t.TempDir())

	if _, err := install(target); err == nil {
		t.Fatal("install accepted an archive with a mismatched digest")
	}
}

func TestLoadExplicitOverrideMustBeRegularFile(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	t.Setenv(EnvLibraryPath, t.TempDir())
	t.Setenv(EnvCacheDir, t.TempDir())
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a directory as the explicit override")
	}
}

func TestLoadExplicitOverrideReportsMissingPath(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	missing := filepath.Join(t.TempDir(), "missing.so")
	t.Setenv(EnvLibraryPath, missing)
	t.Setenv(EnvCacheDir, t.TempDir())
	_, err := Load()
	if err == nil {
		t.Fatal("Load accepted a missing explicit override")
	}
	if !os.IsNotExist(err) && !contains(err.Error(), missing) {
		t.Fatalf("error does not identify the explicit path: %v", err)
	}
}

func TestCurrentTargetsHaveCompleteInventory(t *testing.T) {
	for key, target := range Targets {
		if target.Name == "" || target.Archive == "" || target.LibraryPath == "" || target.LibraryName == "" {
			t.Errorf("%s has an incomplete descriptor: %+v", key, target)
		}
		for name, value := range map[string]string{"archive": target.ArchiveSHA256, "library": target.LibrarySHA256} {
			if len(value) != 64 {
				t.Errorf("%s %s digest = %q, want 64 hex chars", key, name, value)
			}
		}
		if !contains(target.Archive, ReleaseCommit) {
			t.Errorf("%s archive %q does not contain the full release commit", key, target.Archive)
		}
	}
}

func makeArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return raw.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func contains(value, substring string) bool {
	return len(substring) == 0 || (len(value) >= len(substring) && bytes.Contains([]byte(value), []byte(substring)))
}

func fakeTarget(library, archive []byte) Target {
	return Target{
		Name:          "concurrent-target",
		Archive:       "concurrent.tar.gz",
		ArchiveSHA256: sha256Hex(archive),
		LibraryPath:   "lib/libfake.so",
		LibraryName:   "libfake.so",
		LibrarySHA256: sha256Hex(library),
	}
}

func TestInstallConcurrentColdCache(t *testing.T) {
	library := []byte("concurrent runtime bytes")
	archive := makeArchive(t, "lib/libfake.so", library)
	target := fakeTarget(library, archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	t.Setenv(EnvReleaseBaseURL, server.URL)
	t.Setenv(EnvCacheDir, t.TempDir())

	const workers = 8
	var wait sync.WaitGroup
	errs := make(chan error, workers)
	paths := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			path, err := install(target)
			if err != nil {
				errs <- err
				return
			}
			paths <- path
		}()
	}
	wait.Wait()
	close(errs)
	close(paths)
	for err := range errs {
		t.Errorf("concurrent install: %v", err)
	}
	count := 0
	for path := range paths {
		count++
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read concurrent result: %v", err)
			continue
		}
		if !bytes.Equal(got, library) {
			t.Errorf("concurrent result bytes = %q", got)
		}
	}
	if count != workers {
		t.Fatalf("successful concurrent installs = %d, want %d", count, workers)
	}
}

func TestInstallConcurrentProcessesColdCache(t *testing.T) {
	library := []byte("cross-process runtime bytes")
	archive := makeArchive(t, "lib/libfake.so", library)
	target := fakeTarget(library, archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	cache := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}

	const workers = 4
	commands := make([]*exec.Cmd, 0, workers)
	outputs := make([]*bytes.Buffer, 0, workers)
	for i := 0; i < workers; i++ {
		command := exec.Command(exe, "-test.run=^TestInstallCacheHelper$")
		command.Env = append(os.Environ(),
			"LANCEDB_INSTALL_HELPER=1",
			"LANCEDB_INSTALL_TARGET="+target.Name,
			"LANCEDB_INSTALL_ARCHIVE="+target.Archive,
			"LANCEDB_INSTALL_ARCHIVE_SHA="+target.ArchiveSHA256,
			"LANCEDB_INSTALL_LIBRARY="+target.LibraryName,
			"LANCEDB_INSTALL_LIBRARY_SHA="+target.LibrarySHA256,
			"LANCEDB_INSTALL_BASE_URL="+server.URL,
			"LANCEDB_INSTALL_CACHE="+cache,
		)
		output := &bytes.Buffer{}
		command.Stdout = output
		command.Stderr = output
		if err := command.Start(); err != nil {
			t.Fatalf("start worker %d: %v", i, err)
		}
		commands = append(commands, command)
		outputs = append(outputs, output)
	}
	for i, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("worker %d failed: %v\n%s", i, err, outputs[i].String())
		}
	}
	path := filepath.Join(cache, "lancedb-go", "native", ReleaseVersion, target.Name, target.LibraryName)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cross-process result: %v", err)
	}
	if !bytes.Equal(got, library) {
		t.Fatalf("cross-process result bytes = %q", got)
	}
}

func TestInstallCacheHelper(t *testing.T) {
	if os.Getenv("LANCEDB_INSTALL_HELPER") != "1" {
		return
	}
	target := Target{
		Name:          os.Getenv("LANCEDB_INSTALL_TARGET"),
		Archive:       os.Getenv("LANCEDB_INSTALL_ARCHIVE"),
		ArchiveSHA256: os.Getenv("LANCEDB_INSTALL_ARCHIVE_SHA"),
		LibraryPath:   "lib/" + os.Getenv("LANCEDB_INSTALL_LIBRARY"),
		LibraryName:   os.Getenv("LANCEDB_INSTALL_LIBRARY"),
		LibrarySHA256: os.Getenv("LANCEDB_INSTALL_LIBRARY_SHA"),
	}
	if err := os.Setenv(EnvReleaseBaseURL, os.Getenv("LANCEDB_INSTALL_BASE_URL")); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(EnvCacheDir, os.Getenv("LANCEDB_INSTALL_CACHE")); err != nil {
		t.Fatal(err)
	}
	path, err := install(target)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(path)
}

func TestOpenExplicitRelativePathResolvesAbsolute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("relative dlopen path is covered on Linux")
	}
	systemLibrary := "/lib64/libc.so.6"
	if _, err := os.Stat(systemLibrary); err != nil {
		systemLibrary = "/lib/x86_64-linux-gnu/libc.so.6"
	}
	if _, err := os.Stat(systemLibrary); err != nil {
		t.Skip("no system libc fixture")
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	dir := t.TempDir()
	if err := os.Symlink(systemLibrary, filepath.Join(dir, "libc-relative.so")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	library, err := openExplicit("libc-relative.so")
	if err != nil {
		t.Fatalf("open relative explicit path: %v", err)
	}
	if !filepath.IsAbs(library.Path) {
		t.Fatalf("resolved library path = %q, want absolute", library.Path)
	}
	if library.Source != SourceEnvironment {
		t.Fatalf("source = %q, want environment", library.Source)
	}
}

func warmTarget(name string, library []byte) Target {
	return Target{
		Name:          name,
		Archive:       "unavailable.tar.gz",
		ArchiveSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		LibraryPath:   "lib/libfake.so",
		LibraryName:   "libfake.so",
		LibrarySHA256: sha256Hex(library),
	}
}

func writeWarmLibrary(t *testing.T, target Target, library []byte) string {
	t.Helper()
	dir, err := cacheDirFor(target)
	if err != nil {
		t.Fatalf("cache dir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	path := filepath.Join(dir, target.LibraryName)
	if err := os.WriteFile(path, library, 0o600); err != nil {
		t.Fatalf("write warm library: %v", err)
	}
	return path
}

func TestInstallWarmLibraryWorksOffline(t *testing.T) {
	library := []byte("verified warm runtime bytes")
	target := warmTarget("test-warm-offline", library)
	t.Setenv(EnvCacheDir, t.TempDir())
	t.Setenv(EnvReleaseBaseURL, "http://127.0.0.1:1")
	path := writeWarmLibrary(t, target, library)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat warm library: %v", err)
	}

	got, err := install(target)
	if err != nil {
		t.Fatalf("warm install without archive/network: %v", err)
	}
	if got != path {
		t.Fatalf("warm install path = %q, want %q", got, path)
	}
	after, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat after install: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("warm install replaced the verified library")
	}
	bytesAfter, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read after install: %v", err)
	}
	if !bytes.Equal(bytesAfter, library) {
		t.Fatalf("warm library bytes changed: %q", bytesAfter)
	}
}

func TestInstallWarmConcurrentDoesNotReplace(t *testing.T) {
	library := []byte("concurrent warm runtime bytes")
	target := warmTarget("test-warm-concurrent", library)
	t.Setenv(EnvCacheDir, t.TempDir())
	t.Setenv(EnvReleaseBaseURL, "http://127.0.0.1:1")
	path := writeWarmLibrary(t, target, library)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat warm library: %v", err)
	}

	const workers = 8
	var wait sync.WaitGroup
	errs := make(chan error, workers)
	paths := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, err := install(target)
			if err != nil {
				errs <- err
				return
			}
			paths <- got
		}()
	}
	wait.Wait()
	close(errs)
	close(paths)
	for err := range errs {
		t.Errorf("warm concurrent install: %v", err)
	}
	for got := range paths {
		if got != path {
			t.Errorf("warm concurrent path = %q, want %q", got, path)
		}
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after concurrent install: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("concurrent warm install replaced the verified library")
	}
}

func TestDiscoverPreservesVerifiedUnloadableCache(t *testing.T) {
	resetForTest()
	t.Cleanup(resetForTest)
	library := []byte("verified bytes that are not a shared library")
	target := warmTarget("test-unloadable-cache", library)
	t.Setenv(EnvCacheDir, t.TempDir())
	t.Setenv(EnvReleaseBaseURL, "http://127.0.0.1:1")
	t.Setenv(EnvLibraryPath, "")
	path := writeWarmLibrary(t, target, library)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before discover: %v", err)
	}

	key := runtime.GOOS + "/" + runtime.GOARCH
	previous, hadPrevious := Targets[key]
	Targets[key] = target
	t.Cleanup(func() {
		if hadPrevious {
			Targets[key] = previous
		} else {
			delete(Targets, key)
		}
	})

	_, err = discover()
	if err == nil {
		t.Fatal("discover loaded a non-library file")
	}
	if !contains(err.Error(), "verified but could not be loaded") {
		t.Fatalf("discover did not report the cache load failure: %v", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("verified cache file was removed: %v", readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("verified cache file changed: %q", after)
	}
}
