// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestInstallExtractsAndReusesCache(t *testing.T) {
	library := []byte("fake runtime library bytes")
	target := fakeTarget("test-target")
	useBundle(t, testBundle(t, target, library))
	cache := t.TempDir()
	t.Setenv(EnvCacheDir, cache)

	path, err := install(target)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	want := filepath.Join(cache, "lancedb-go", "native", ReleaseVersion, target.Name, target.LibraryName)
	if path != want {
		t.Fatalf("install path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed library: %v", err)
	}
	if !bytes.Equal(got, library) {
		t.Fatalf("installed bytes differ: %q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat installed library: %v", err)
		}
		if info.Mode().Perm() != 0o500 {
			t.Fatalf("installed mode = %s, want 0500", info.Mode().Perm())
		}
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before reuse: %v", err)
	}
	again, err := install(target)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if again != path {
		t.Fatalf("second install path = %q, want %q", again, path)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after reuse: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("warm install replaced the verified library")
	}
	leftover, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(leftover) != 0 {
		t.Fatalf("temporary files left in cache: %v", leftover)
	}
}

func TestInstallReplacesCorruptCache(t *testing.T) {
	library := []byte("correct runtime bytes")
	target := fakeTarget("test-corrupt-cache")
	useBundle(t, testBundle(t, target, library))
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

func TestInstallRejectsCorruptPayload(t *testing.T) {
	library := []byte("library bytes for corruption")
	target := fakeTarget("test-corrupt-payload")
	bundle := testBundle(t, target, library)
	t.Setenv(EnvCacheDir, t.TempDir())

	cases := []struct {
		name        string
		mutate      func(*Bundle)
		wantDigest  bool
		wantMessage string
	}{
		{
			name: "compressed digest",
			mutate: func(b *Bundle) {
				b.CompressedSHA256 = strings.Repeat("0", 64)
			},
			wantDigest: true,
		},
		{
			name: "uncompressed digest",
			mutate: func(b *Bundle) {
				b.UncompressedSHA256 = strings.Repeat("0", 64)
			},
			wantDigest: true,
		},
		{
			name: "decompressed size",
			mutate: func(b *Bundle) {
				b.UncompressedSize++
			},
			wantDigest: true,
		},
		{
			name: "compressed size metadata",
			mutate: func(b *Bundle) {
				b.CompressedSize++
			},
			wantMessage: "payload compressed size",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corrupt := *bundle
			tc.mutate(&corrupt)
			useBundle(t, &corrupt)

			_, err := install(target)
			if err == nil {
				t.Fatal("install accepted a corrupt compiled-in payload")
			}
			if tc.wantDigest && !errors.Is(err, errDigestMismatch) {
				t.Fatalf("install error = %v, want a digest mismatch", err)
			}
			if tc.wantMessage != "" && !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("install error = %v, want a complaint containing %q", err, tc.wantMessage)
			}
			dir, dirErr := cacheDirFor(target)
			if dirErr != nil {
				t.Fatalf("cache dir: %v", dirErr)
			}
			if _, statErr := os.Stat(filepath.Join(dir, target.LibraryName)); !os.IsNotExist(statErr) {
				t.Fatalf("corrupt payload was published to the cache: %v", statErr)
			}
			leftover, globErr := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
			if globErr != nil {
				t.Fatalf("glob temporary files: %v", globErr)
			}
			if len(leftover) != 0 {
				t.Fatalf("temporary files left in cache: %v", leftover)
			}
		})
	}
}

func TestInstallWithoutCompiledPayload(t *testing.T) {
	previous := registeredBundle
	registeredBundle = nil
	t.Cleanup(func() { registeredBundle = previous })

	target := fakeTarget("test-no-payload")
	t.Setenv(EnvCacheDir, t.TempDir())
	_, err := install(target)
	if err == nil {
		t.Fatal("install succeeded without a compiled-in payload")
	}
	if !strings.Contains(err.Error(), "no compiled-in native payload") {
		t.Fatalf("install error = %v, want a missing-payload complaint", err)
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
		if target.Name == "" || target.LibraryName == "" {
			t.Errorf("%s has an incomplete descriptor: %+v", key, target)
		}
		if want := strings.ReplaceAll(key, "/", "-"); target.Name != want {
			t.Errorf("%s target name = %q, want %q", key, target.Name, want)
		}
	}
}

func TestCompiledSelectorMatchesCurrentTarget(t *testing.T) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	if _, ok := Targets[key]; !ok {
		t.Skipf("no supported native payload for %s", key)
	}
	target, err := CurrentTarget()
	if err != nil {
		t.Fatalf("current target: %v", err)
	}
	bundle, err := bundleFor(target)
	if err != nil {
		t.Fatalf("compiled-in selector bundle: %v", err)
	}
	if want := ArtifactModulePrefix + "/" + target.Name; bundle.Module != want {
		t.Fatalf("bundle module = %q, want %q", bundle.Module, want)
	}
	if bundle.Target != key {
		t.Fatalf("bundle target = %q, want %q", bundle.Target, key)
	}
	if bundle.LibraryName != target.LibraryName {
		t.Fatalf("bundle library = %q, want %q", bundle.LibraryName, target.LibraryName)
	}
}

func TestInstallConcurrentColdCache(t *testing.T) {
	library := []byte("concurrent runtime bytes")
	target := fakeTarget("concurrent-target")
	useBundle(t, testBundle(t, target, library))
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
	target := fakeTarget("cross-process-target")
	bundle := testBundle(t, target, library)
	payload := filepath.Join(t.TempDir(), "payload.zst")
	if err := os.WriteFile(payload, bundle.Compressed, 0o600); err != nil {
		t.Fatalf("write payload fixture: %v", err)
	}
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
			"LANCEDB_INSTALL_LIBRARY="+target.LibraryName,
			"LANCEDB_INSTALL_PAYLOAD="+payload,
			"LANCEDB_INSTALL_COMPRESSED_SIZE="+strconv.FormatInt(bundle.CompressedSize, 10),
			"LANCEDB_INSTALL_COMPRESSED_SHA="+bundle.CompressedSHA256,
			"LANCEDB_INSTALL_UNCOMPRESSED_SIZE="+strconv.FormatInt(bundle.UncompressedSize, 10),
			"LANCEDB_INSTALL_UNCOMPRESSED_SHA="+bundle.UncompressedSHA256,
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
	payload, err := os.ReadFile(os.Getenv("LANCEDB_INSTALL_PAYLOAD"))
	if err != nil {
		t.Fatal(err)
	}
	compressedSize, err := strconv.ParseInt(os.Getenv("LANCEDB_INSTALL_COMPRESSED_SIZE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	uncompressedSize, err := strconv.ParseInt(os.Getenv("LANCEDB_INSTALL_UNCOMPRESSED_SIZE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{
		Name:        os.Getenv("LANCEDB_INSTALL_TARGET"),
		LibraryName: os.Getenv("LANCEDB_INSTALL_LIBRARY"),
	}
	registeredBundle = &Bundle{
		Module:             ArtifactModulePrefix + "/" + target.Name,
		Target:             runtime.GOOS + "/" + runtime.GOARCH,
		LibraryName:        target.LibraryName,
		Compressed:         payload,
		CompressedSize:     compressedSize,
		CompressedSHA256:   os.Getenv("LANCEDB_INSTALL_COMPRESSED_SHA"),
		UncompressedSize:   uncompressedSize,
		UncompressedSHA256: os.Getenv("LANCEDB_INSTALL_UNCOMPRESSED_SHA"),
		SourceRepo:         "lancedb/lancedb-c",
		SourceRelease:      ReleaseTag,
		SourceCommit:       strings.Repeat("a", 40),
		HeaderSHA256:       strings.Repeat("b", 64),
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

func TestInstallWarmLibraryWorksOffline(t *testing.T) {
	library := []byte("verified warm runtime bytes")
	target := fakeTarget("test-warm-offline")
	bundle := testBundle(t, target, library)
	// A warm entry must not consult the payload. Point the digest at a value
	// the temp directory does not contain so any extraction attempt fails.
	bundle.CompressedSHA256 = strings.Repeat("0", 64)
	useBundle(t, bundle)
	t.Setenv(EnvCacheDir, t.TempDir())
	path := writeWarmLibrary(t, target, library)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat warm library: %v", err)
	}

	got, err := install(target)
	if err != nil {
		t.Fatalf("warm install without payload extraction: %v", err)
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
	target := fakeTarget("test-warm-concurrent")
	useBundle(t, testBundle(t, target, library))
	t.Setenv(EnvCacheDir, t.TempDir())
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
	target := fakeTarget("test-unloadable-cache")
	useBundle(t, testBundle(t, target, library))
	t.Setenv(EnvCacheDir, t.TempDir())
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

func fakeTarget(name string) Target {
	return Target{Name: name, LibraryName: "libfake.so"}
}

// testBundle builds a self-consistent bundle fixture. The module path matches
// target.Name so bundleFor accepts it; the callers that exercise mismatches
// mutate the returned value.
func testBundle(t *testing.T, target Target, library []byte) *Bundle {
	t.Helper()
	compressed := zstdCompress(t, library)
	return &Bundle{
		Module:             ArtifactModulePrefix + "/" + target.Name,
		Target:             runtime.GOOS + "/" + runtime.GOARCH,
		LibraryName:        target.LibraryName,
		Compressed:         compressed,
		CompressedSize:     int64(len(compressed)),
		CompressedSHA256:   sha256Hex(compressed),
		UncompressedSize:   int64(len(library)),
		UncompressedSHA256: sha256Hex(library),
		SourceRepo:         "lancedb/lancedb-c",
		SourceRelease:      ReleaseTag,
		SourceCommit:       strings.Repeat("a", 40),
		HeaderSHA256:       strings.Repeat("b", 64),
	}
}

func useBundle(t *testing.T, bundle *Bundle) {
	t.Helper()
	previous := registeredBundle
	registeredBundle = bundle
	t.Cleanup(func() { registeredBundle = previous })
}

func zstdCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("write zstd payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}
	return buf.Bytes()
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

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func contains(value, substring string) bool {
	return len(substring) == 0 || strings.Contains(value, substring)
}
