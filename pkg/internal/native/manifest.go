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
	ReleaseTag     = "v0.1.0-preview.3"
	ReleaseVersion = "0.1.0-preview.3"
	ReleaseCommit  = "db285fa312ca922ded00cb503b12f6df629932ac"
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
		ArchiveSHA256: "8368dddbbe65e23f945eebb05d3408a4bc98a68bbd4bd89566617b150b84861d",
		LibraryPath:   "lib/liblancedb.so",
		LibraryName:   "liblancedb.so",
		LibrarySHA256: "9b15ae2d1b0e17879c69f6b6b8419e39bb926a91a25f0ce89ef8c3d9ed0166ae",
	},
	"linux/arm64": {
		Name:          "linux-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-linux-arm64.tar.gz",
		ArchiveSHA256: "acbe8f5015af081c626f1df532999f7b27ab5306dcffb7664462fc0cea73d11a",
		LibraryPath:   "lib/liblancedb.so",
		LibraryName:   "liblancedb.so",
		LibrarySHA256: "b496f1736dd69db744d541cf5dc2ce3fbfcd35ae02a7793c427bcfbc559177da",
	},
	"darwin/arm64": {
		Name:          "darwin-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-darwin-arm64.tar.gz",
		ArchiveSHA256: "9d72e7ec9c3f70d5b681be2b0b2563d9e210157ab33f9f0b50bf256762f721d3",
		LibraryPath:   "lib/liblancedb.dylib",
		LibraryName:   "liblancedb.dylib",
		LibrarySHA256: "322573a2fc1d1d36b85b8b411603262ee2392a05bf858a4a1ca10e177a129cc2",
	},
	"windows/amd64": {
		Name:          "windows-amd64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-windows-amd64.tar.gz",
		ArchiveSHA256: "8118922136f000cd0758635bf63cd8051a1f15042945a8219a82e54500060d85",
		LibraryPath:   "lib/lancedb.dll",
		LibraryName:   "lancedb.dll",
		LibrarySHA256: "97b0ab0fe1a787b6d9f7959a629cb41919161f0e97d0652e4fd5c0576c682e8a",
	},
	"windows/arm64": {
		Name:          "windows-arm64",
		Archive:       "lancedb-c-" + ReleaseCommit + "-windows-arm64.tar.gz",
		ArchiveSHA256: "5975d7b038d0edc2739db486d4f5e22bbfcd052666bca41e56870807b77e12fa",
		LibraryPath:   "lib/lancedb.dll",
		LibraryName:   "lancedb.dll",
		LibrarySHA256: "39eb2d0d1703fedf6b0733aaaf83dcd851cf27ee69a6a86b5fd2e8f20d7fe3b8",
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
