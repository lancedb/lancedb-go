// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"errors"
	"fmt"
	"math"
	"runtime"
)

// Bundle mirrors one per-target companion module's payload and packaging
// metadata. A build-tagged selector in this package registers exactly one
// Bundle during package initialization. The payload is data only: importing the
// artifact module neither links nor executes native code, and no library is
// loaded until install materializes it.
type Bundle struct {
	// Module is the import path of the companion module that supplied the
	// payload. It is checked against the selected target and used in
	// diagnostics.
	Module string
	// Target is the Go target the payload was packaged for, as GOOS/GOARCH.
	Target string
	// LibraryName is the platform shared-library filename for Target.
	LibraryName string
	// Compressed is a zstd frame containing the shared library bytes.
	Compressed []byte
	// CompressedSize is len(Compressed) as recorded at packaging time.
	CompressedSize int64
	// CompressedSHA256 is the lowercase SHA-256 of Compressed.
	CompressedSHA256 string
	// UncompressedSize is the exact shared-library size.
	UncompressedSize int64
	// UncompressedSHA256 is the lowercase SHA-256 of the shared library.
	UncompressedSHA256 string
	// UpstreamRepo and UpstreamCommit identify the lancedb-c source the
	// payload was packaged from. UpstreamCommit must equal ReleaseCommit.
	UpstreamRepo   string
	UpstreamCommit string
	// HeaderSHA256 is the SHA-256 of the C header used during packaging.
	// These provenance fields are packaging-time claims, not properties
	// verified from the shared library.
	HeaderSHA256 string
}

// registeredBundle is the descriptor installed by a build-tagged selector. It
// is written during package initialization, before any Load call.
var registeredBundle *Bundle

// registerBundle installs the descriptor linked in by a build-tagged selector.
// Selectors are mutually exclusive per build, so a second registration is a
// packaging bug; panic instead of silently choosing one payload.
func registerBundle(b Bundle) {
	if registeredBundle != nil {
		panic("native: more than one compiled-in artifact bundle registered; link exactly one per-target selector")
	}
	registeredBundle = &b
}

// bundleFor returns the compiled-in bundle for target after checking that it
// matches the running build and the supported target inventory.
func bundleFor(target Target) (*Bundle, error) {
	bundle := registeredBundle
	if bundle == nil {
		return nil, fmt.Errorf("this build has no compiled-in native payload for %s; build for a supported target or set %s to a compatible lancedb-c library", runtime.GOOS+"/"+runtime.GOARCH, EnvLibraryPath)
	}
	if err := bundle.validate(); err != nil {
		return nil, fmt.Errorf("compiled-in native payload: %w", err)
	}
	if want := ArtifactModulePrefix + "/" + target.Name; bundle.Module != want {
		return nil, fmt.Errorf("compiled-in native payload module is %q, want %q", bundle.Module, want)
	}
	if bundle.LibraryName != target.LibraryName {
		return nil, fmt.Errorf("compiled-in native payload for %s names library %q, want %q", bundle.Target, bundle.LibraryName, target.LibraryName)
	}
	if bundle.UpstreamCommit != ReleaseCommit {
		return nil, fmt.Errorf("compiled-in native payload was packaged from %s, want %s", bundle.UpstreamCommit, ReleaseCommit)
	}
	return bundle, nil
}

func (b *Bundle) validate() error {
	wantTarget := runtime.GOOS + "/" + runtime.GOARCH
	if b.Target != wantTarget {
		return fmt.Errorf("payload target is %q, want %q", b.Target, wantTarget)
	}
	if b.Module == "" {
		return errors.New("payload module path is empty")
	}
	if b.LibraryName == "" {
		return errors.New("payload library name is empty")
	}
	if b.CompressedSize <= 0 || b.CompressedSize != int64(len(b.Compressed)) {
		return fmt.Errorf("payload compressed size is %d, payload has %d bytes", b.CompressedSize, len(b.Compressed))
	}
	if !isLowerHex(b.CompressedSHA256, 64) {
		return fmt.Errorf("payload compressed SHA-256 %q is not 64 lowercase hex characters", b.CompressedSHA256)
	}
	if b.UncompressedSize <= 0 || b.UncompressedSize == math.MaxInt64 {
		return fmt.Errorf("payload uncompressed size %d is invalid", b.UncompressedSize)
	}
	if !isLowerHex(b.UncompressedSHA256, 64) {
		return fmt.Errorf("payload uncompressed SHA-256 %q is not 64 lowercase hex characters", b.UncompressedSHA256)
	}
	if b.UpstreamRepo == "" {
		return errors.New("payload upstream repository is empty")
	}
	if !isLowerHex(b.UpstreamCommit, 40) {
		return fmt.Errorf("payload upstream commit %q is not a 40-character lowercase hex commit", b.UpstreamCommit)
	}
	if !isLowerHex(b.HeaderSHA256, 64) {
		return fmt.Errorf("payload header SHA-256 %q is not 64 lowercase hex characters", b.HeaderSHA256)
	}
	return nil
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
