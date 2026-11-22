// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"context"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

// Connect establishes a connection to a LanceDB database with the published
// v0.1.2 options. Region and ReadConsistencyInterval remain ignored, as they
// were in v0.1.2. Structured S3 settings are applied to this connection rather
// than mutating process environment variables.
func Connect(ctx context.Context, uri string, options *contracts.ConnectionOptions) (contracts.IConnection, error) {
	return internal.Connect(ctx, uri, options)
}

// ConnectWithStorageOptions opens a connection with a flat storage-option map.
// This is the additive path for options the v0.1.2 structs cannot express. Use
// contracts.Storage* constants for well-known keys, including aws_profile,
// aws_region and dynamodb_endpoint.
func ConnectWithStorageOptions(ctx context.Context, uri string, storageOptions map[string]string) (contracts.IConnection, error) {
	return internal.ConnectWithStorageOptions(ctx, uri, storageOptions)
}

// RuntimeVersion reads the version reported by the loaded lancedb-c library.
// The binding requires this to equal the preview it was built against.
func RuntimeVersion() (string, error) {
	api, err := native.Load()
	if err != nil {
		return "", err
	}
	return api.Version(), nil
}

// NativeLibrary describes the loaded native library. It is useful for
// diagnostics and for verifying explicit override or cache discovery.
type NativeLibrary struct {
	Path    string
	Source  string
	Version string
}

// LoadedNativeLibrary returns the current native-library state without loading
// a new library.
func LoadedNativeLibrary() NativeLibrary {
	state := native.Observe()
	return NativeLibrary{
		Path:    state.Path,
		Source:  string(state.Source),
		Version: state.Version,
	}
}

// InstallNativeLibrary downloads, verifies and extracts the pinned lancedb-c
// artifact for the running target into the private cache. It returns the
// runtime library path. It is useful for pre-seeding an offline environment or
// a CI cache; Load performs the same installation lazily.
func InstallNativeLibrary() (string, error) {
	return native.Install()
}
