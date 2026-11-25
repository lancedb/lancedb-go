// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

import (
	"fmt"
	"runtime"
)

// The native shared library is delivered as one compiled-in companion module
// per target: go.lancedb.io/lancedb-go/artifacts/<goos>-<goarch>. The root
// module requires all five and the build-tagged selectors in this package link
// exactly the one that matches the build. The payload, its sizes and its
// digests live in the artifact module; the constants here name the lancedb-c
// release the payloads were built from and the runtime version this binding
// was compiled against.
const (
	// ReleaseTag is the lancedb-c release the companion payloads were built
	// from.
	ReleaseTag = "v0.1.0-preview.5"
	// ReleaseCommit is the peeled lancedb-c commit for ReleaseTag. Companion
	// payloads record it as UpstreamCommit and the loader requires a match.
	ReleaseCommit = "3106de158b8da5dcb47e2a4acc820ed1b74b3e01"
	// ReleaseVersion is the version lancedb_c_version() must report for the
	// loaded library.
	ReleaseVersion = "0.1.0-preview.5"
	// ArtifactModulePrefix is the import prefix of the per-target companion
	// modules.
	ArtifactModulePrefix = "go.lancedb.io/lancedb-go/artifacts"
)

// Target identifies one supported Go target.
type Target struct {
	// Name is the companion module suffix and the private cache directory
	// name, for example "linux-amd64".
	Name string
	// LibraryName is the runtime library filename used in the private cache.
	LibraryName string
}

// Targets is the supported target inventory. lancedb-c publishes Linux and
// Windows on amd64 and arm64, and macOS on Apple Silicon. Intel macOS is not
// supported.
var Targets = map[string]Target{
	"linux/amd64":   {Name: "linux-amd64", LibraryName: "liblancedb.so"},
	"linux/arm64":   {Name: "linux-arm64", LibraryName: "liblancedb.so"},
	"darwin/arm64":  {Name: "darwin-arm64", LibraryName: "liblancedb.dylib"},
	"windows/amd64": {Name: "windows-amd64", LibraryName: "lancedb.dll"},
	"windows/arm64": {Name: "windows-arm64", LibraryName: "lancedb.dll"},
}

// CurrentTarget returns the supported target for the running Go build.
func CurrentTarget() (Target, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	t, ok := Targets[key]
	if !ok {
		return Target{}, fmt.Errorf("lancedb-go supports no native payload for %s; supported targets: linux/amd64, linux/arm64, darwin/arm64, windows/amd64, windows/arm64", key)
	}
	return t, nil
}
