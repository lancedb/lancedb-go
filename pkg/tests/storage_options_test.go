// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func TestStorageOptionsBasic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_storage_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Connect with nil options", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		if conn.IsClosed() {
			t.Fatal("Connection should not be closed")
		}
	})

	t.Run("Connect with empty ConnectionOptions", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})

	t.Run("Connect with empty map", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{
			StorageOptions: map[string]string{},
		})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})
}

func TestStorageOptionsJSONSerialization(t *testing.T) {
	t.Run("empty map marshals to {}", func(t *testing.T) {
		opts := map[string]string{}
		data, err := json.Marshal(opts)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}
		if string(data) != "{}" {
			t.Fatalf("Expected {}, got %s", string(data))
		}
	})

	t.Run("S3 keys produce flat JSON", func(t *testing.T) {
		opts := map[string]string{
			contracts.StorageAccessKeyID:     "AKIA...",
			contracts.StorageSecretAccessKey: "wJalr...",
			contracts.StorageRegion:          "us-east-1",
		}
		data, err := json.Marshal(opts)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		// Verify it round-trips correctly
		var parsed map[string]string
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if parsed["access_key_id"] != "AKIA..." {
			t.Fatalf("Expected AKIA..., got %s", parsed["access_key_id"])
		}
		if parsed["region"] != "us-east-1" {
			t.Fatalf("Expected us-east-1, got %s", parsed["region"])
		}
	})

	t.Run("mixed backend keys serialize correctly", func(t *testing.T) {
		opts := map[string]string{
			contracts.StorageAccessKeyID:      "key",
			contracts.StorageAzureAccountName: "account",
			contracts.StorageGCSBucket:        "mybucket",
			contracts.StorageAllowHTTP:        "true",
		}
		data, err := json.Marshal(opts)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var parsed map[string]string
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		if len(parsed) != 4 {
			t.Fatalf("Expected 4 keys, got %d", len(parsed))
		}
	})
}

func TestStorageKeyConstants(t *testing.T) {
	t.Run("all constants are non-empty", func(t *testing.T) {
		constants := []string{
			contracts.StorageAccessKeyID,
			contracts.StorageSecretAccessKey,
			contracts.StorageSessionToken,
			contracts.StorageRegion,
			contracts.StorageEndpoint,
			contracts.StorageAWSEndpoint,
			contracts.StorageVirtualHostedStyleRequest,
			contracts.StorageUnsignedPayload,
			contracts.StorageConditionalPut,
			contracts.StorageCopyIfNotExists,
			contracts.StorageS3Express,
			contracts.StorageRoleArn,
			contracts.StorageRoleSessionName,
			contracts.StorageWebIdentityTokenFile,
			contracts.StorageDefaultRegion,
			contracts.StorageBucket,
			contracts.StorageSkipSignature,
			contracts.StorageDisableTagging,
			contracts.StorageRequestPayer,
			contracts.StorageGCSServiceAccount,
			contracts.StorageGCSServiceAccountKey,
			contracts.StorageGCSApplicationCredentials,
			contracts.StorageGCSBucket,
			contracts.StorageAzureAccountName,
			contracts.StorageAzureAccessKey,
			contracts.StorageAzureSASToken,
			contracts.StorageAzureTenantID,
			contracts.StorageAzureClientID,
			contracts.StorageAzureClientSecret,
			contracts.StorageAzureAuthorityID,
			contracts.StorageAzureContainerName,
			contracts.StorageAzureEndpoint,
			contracts.StorageAzureUseFabricEndpoint,
			contracts.StorageAzureMSIEndpoint,
			contracts.StorageAzureUseAzureCLI,
			contracts.StorageAllowHTTP,
			contracts.StorageAllowInvalidCertificates,
			contracts.StorageConnectTimeout,
			contracts.StorageTimeout,
			contracts.StorageUserAgent,
			contracts.StorageProxyURL,
		}

		for _, c := range constants {
			if c == "" {
				t.Fatal("Found empty constant")
			}
		}
	})

	t.Run("no duplicate values", func(t *testing.T) {
		constants := []string{
			contracts.StorageAccessKeyID,
			contracts.StorageSecretAccessKey,
			contracts.StorageSessionToken,
			contracts.StorageRegion,
			contracts.StorageEndpoint,
			contracts.StorageAWSEndpoint,
			contracts.StorageVirtualHostedStyleRequest,
			contracts.StorageUnsignedPayload,
			contracts.StorageConditionalPut,
			contracts.StorageCopyIfNotExists,
			contracts.StorageS3Express,
			contracts.StorageRoleArn,
			contracts.StorageRoleSessionName,
			contracts.StorageWebIdentityTokenFile,
			contracts.StorageDefaultRegion,
			contracts.StorageBucket,
			contracts.StorageSkipSignature,
			contracts.StorageDisableTagging,
			contracts.StorageRequestPayer,
			contracts.StorageGCSServiceAccount,
			contracts.StorageGCSServiceAccountKey,
			contracts.StorageGCSApplicationCredentials,
			contracts.StorageGCSBucket,
			contracts.StorageAzureAccountName,
			contracts.StorageAzureAccessKey,
			contracts.StorageAzureSASToken,
			contracts.StorageAzureTenantID,
			contracts.StorageAzureClientID,
			contracts.StorageAzureClientSecret,
			contracts.StorageAzureAuthorityID,
			contracts.StorageAzureContainerName,
			contracts.StorageAzureEndpoint,
			contracts.StorageAzureUseFabricEndpoint,
			contracts.StorageAzureMSIEndpoint,
			contracts.StorageAzureUseAzureCLI,
			contracts.StorageAllowHTTP,
			contracts.StorageAllowInvalidCertificates,
			contracts.StorageConnectTimeout,
			contracts.StorageTimeout,
			contracts.StorageUserAgent,
			contracts.StorageProxyURL,
		}

		seen := make(map[string]bool)
		for _, c := range constants {
			if seen[c] {
				t.Fatalf("Duplicate constant value: %s", c)
			}
			seen[c] = true
		}
	})
}

func TestConnectWithStorageOptions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_storage_opts_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("S3-style options with local path", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{
			StorageOptions: map[string]string{
				contracts.StorageAccessKeyID:     "test-key",
				contracts.StorageSecretAccessKey: "test-secret",
				contracts.StorageRegion:          "us-east-1",
			},
		})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})

	t.Run("GCS-style options with local path", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{
			StorageOptions: map[string]string{
				contracts.StorageGCSServiceAccount: "/path/to/sa.json",
			},
		})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})

	t.Run("Azure-style options with local path", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{
			StorageOptions: map[string]string{
				contracts.StorageAzureAccountName: "testaccount",
				contracts.StorageAzureAccessKey:   "testkey",
			},
		})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})

	t.Run("mixed backend options with local path", func(t *testing.T) {
		conn, err := lancedb.Connect(context.Background(), tempDir, &contracts.ConnectionOptions{
			StorageOptions: map[string]string{
				contracts.StorageAccessKeyID:      "key",
				contracts.StorageAzureAccountName: "account",
				contracts.StorageAllowHTTP:        "true",
			},
		})
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()
	})
}
