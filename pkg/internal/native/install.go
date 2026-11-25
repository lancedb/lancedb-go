// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/klauspost/compress/zstd"
)

// install returns the verified cache path for target. On a cold or damaged
// entry it decompresses the compiled-in companion payload for the running
// target; a verified entry is reused without touching its inode and without
// decompressing the payload again.
//
// The payload comes from the per-target artifact module linked by a
// build-tagged selector, so installation never uses the network at runtime.
func install(target Target) (string, error) {
	bundle, err := bundleFor(target)
	if err != nil {
		return "", err
	}

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
	usable, err := prepareCachedLibrary(libPath, bundle.UncompressedSHA256)
	if err != nil {
		return "", err
	}
	if usable {
		// A verified library is a complete cache entry. Do not decompress the
		// payload again and do not replace its inode.
		return libPath, nil
	}

	if err := extractBundle(bundle, dir, libPath); err != nil {
		return "", err
	}
	return libPath, nil
}

// extractBundle verifies the compiled-in payload and publishes the library
// into dir. The caller holds the per-target cache lock.
func extractBundle(bundle *Bundle, dir, destination string) error {
	if sum := sha256.Sum256(bundle.Compressed); hex.EncodeToString(sum[:]) != bundle.CompressedSHA256 {
		return fmt.Errorf("compiled-in %s payload: %w: sha256 is %s, want %s", bundle.Module, errDigestMismatch, hex.EncodeToString(sum[:]), bundle.CompressedSHA256)
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(destination)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary library in %s: %w", dir, err)
	}
	tmp := tmpFile.Name()
	published := false
	defer func() {
		if published {
			return
		}
		tmpFile.Close()
		os.Remove(tmp)
	}()

	decoder, err := zstd.NewReader(bytes.NewReader(bundle.Compressed))
	if err != nil {
		return fmt.Errorf("open compiled-in %s payload: %w", bundle.Module, err)
	}
	defer decoder.Close()

	h := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, h), io.LimitReader(decoder, bundle.UncompressedSize+1))
	if err != nil {
		return fmt.Errorf("decompress compiled-in %s payload: %w", bundle.Module, err)
	}
	if written != bundle.UncompressedSize {
		return fmt.Errorf("compiled-in %s payload: %w: decompressed %d bytes, want %d", bundle.Module, errDigestMismatch, written, bundle.UncompressedSize)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != bundle.UncompressedSHA256 {
		return fmt.Errorf("compiled-in %s payload: %w: library sha256 is %s, want %s", bundle.Module, errDigestMismatch, got, bundle.UncompressedSHA256)
	}
	if err := os.Chmod(tmp, 0o500); err != nil && !isWindows() {
		return fmt.Errorf("protect extracted library: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync extracted library: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary library %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, destination); err != nil {
		// Another installer may have published the same verified bytes before
		// this process acquired the shared cache lock.
		if verifyFile(destination, bundle.UncompressedSHA256) == nil {
			return nil
		}
		return fmt.Errorf("publish cached library %s: %w", destination, err)
	}
	published = true
	return nil
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

// Install verifies and extracts the compiled-in companion payload for the
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
