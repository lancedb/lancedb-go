// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
