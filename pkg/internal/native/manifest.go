// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"fmt"
	"runtime"
)

// The native library is distributed by lancedb-c. Keep these values in sync
// with the published release inventory; the full commit is part of every
// asset name and must not be shortened or guessed.
const (
	ReleaseTag     = "v0.1.0-preview.2"
	ReleaseVersion = "0.1.0-preview.2"
	ReleaseCommit  = "250aa95153848c6fc6f4af571af70e979fb89ce3"
	ReleaseRepo    = "lancedb/lancedb-c"
)

// Target describes one published archive for a Go target.
type Target struct {
	// Name is the archive suffix used by lancedb-c, for example "linux-amd64".
	Name string
	// Archive is the exact published asset name.
	Archive string
	// ArchiveSHA256 is the digest published in SHA256SUMS.
	ArchiveSHA256 string
	// LibraryPath is the runtime library path inside the archive.
	LibraryPath string
	// LibraryName is the filename used in the private cache.
	LibraryName string
	// LibrarySHA256 is the digest of the extracted runtime library.
	LibrarySHA256 string
}

// Targets is the published inventory. lancedb-c publishes Linux and Windows on
// amd64 and arm64, and macOS on Apple Silicon. Intel macOS is not supported.
var Targets = map[string]Target{
	"linux/amd64": {
		Name:          "linux-amd64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-linux-amd64.tar.gz",
		ArchiveSHA256: "bd1fd0688dba257887eafe4b855331bcaf0d0fc7209292cf90bb2e9fe7d0e6ee",
		LibraryPath:   "lib/liblancedb.so",
		LibraryName:   "liblancedb.so",
		LibrarySHA256: "6731944f6c4c85201e127f1e83f41275c45233397504cc05a1e166d276c2a9ef",
	},
	"linux/arm64": {
		Name:          "linux-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-linux-arm64.tar.gz",
		ArchiveSHA256: "3812530e38a629505b83db96fa1d42fceb10425c9cbd165f8fdb50a1f1a56df6",
		LibraryPath:   "lib/liblancedb.so",
		LibraryName:   "liblancedb.so",
		LibrarySHA256: "9b85eebec8ebff3ea8dc2cfb9053564cd8c87da672e5aa4122c469a89b6cfab8",
	},
	"darwin/arm64": {
		Name:          "darwin-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-darwin-arm64.tar.gz",
		ArchiveSHA256: "d3974f838854b91a327591bb60c109da66bd101a5ac1254cfa6be278bd9171ac",
		LibraryPath:   "lib/liblancedb.dylib",
		LibraryName:   "liblancedb.dylib",
		LibrarySHA256: "d9fa3f36164f26ffe28cecab8f8fdc4e5e60a0beb2f79903a0bff49b95ff14e8",
	},
	"windows/amd64": {
		Name:          "windows-amd64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-windows-amd64.tar.gz",
		ArchiveSHA256: "a65f762a3a95537064a5052ec9b79a199a22433e39ae21f4a2571f10b2ce425f",
		LibraryPath:   "lib/lancedb.dll",
		LibraryName:   "lancedb.dll",
		LibrarySHA256: "2cf1a6e6100927c90845a0288d57fb5fc836ae0a276ec603b752a37da6fec467",
	},
	"windows/arm64": {
		Name:          "windows-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-windows-arm64.tar.gz",
		ArchiveSHA256: "17d05ad2d9e88a470f13f7e455796ee2189ebd7aa4bec087696cbe91e636f826",
		LibraryPath:   "lib/lancedb.dll",
		LibraryName:   "lancedb.dll",
		LibrarySHA256: "67f4685894301d9b9acc5cc2e6b13a8b858b3e53dda5b51b555275ff151c6668",
	},
}

// CurrentTarget returns the published artifact for the running Go target.
func CurrentTarget() (Target, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	t, ok := Targets[key]
	if !ok {
		return Target{}, fmt.Errorf("lancedb-c %s publishes no artifact for %s; supported targets: linux/amd64, linux/arm64, darwin/arm64, windows/amd64, windows/arm64", ReleaseTag, key)
	}
	return t, nil
}
