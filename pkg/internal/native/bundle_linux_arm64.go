// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//go:build linux && arm64

package native

import (
	artifact "go.lancedb.io/lancedb-go/artifacts/linux-arm64"
)

func init() {
	registerBundle(Bundle{
		Module:             "go.lancedb.io/lancedb-go/artifacts/linux-arm64",
		Target:             artifact.Target,
		LibraryName:        artifact.LibraryName,
		Compressed:         artifact.Payload,
		CompressedSize:     artifact.CompressedSize,
		CompressedSHA256:   artifact.CompressedSHA256,
		UncompressedSize:   artifact.UncompressedSize,
		UncompressedSHA256: artifact.UncompressedSHA256,
		UpstreamRepo:       artifact.UpstreamRepo,
		UpstreamCommit:     artifact.UpstreamCommit,
		HeaderSHA256:       artifact.HeaderSHA256,
	})
}
