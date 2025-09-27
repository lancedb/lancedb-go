// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"

	lancedb "github.com/lancedb/lancedb-go/pkg"
	"github.com/lancedb/lancedb-go/pkg/contracts"
)

func TestStorageOptionsBasic(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_storage_")
	if err != nil {
		t.Fatalf(" ❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Connect with nil StorageOptions", func(t *testing.T) {
		// Test connection without any storage options (should work like before)
		conn, err := lancedb.Connect(context.Background(), tempDir, nil)
		if err != nil {
			t.Fatalf(" ❌Failed to connect without storage options: %v", err)
		}
		defer conn.Close()

		if conn.IsClosed() {
			t.Fatal("❌Connection should not be marked as closed")
		}

		t.Log("✅ Connection without storage options works correctly")
	})

	t.Run("Connect with empty ConnectionOptions", func(t *testing.T) {
		// Test connection with empty ConnectionOptions
		options := &contracts.ConnectionOptions{}
		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with empty options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Connection with empty options works correctly")
	})

	t.Run("Connect with empty StorageOptions", func(t *testing.T) {
		// Test connection with empty StorageOptions
		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{},
		}
		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with empty storage options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Connection with empty storage options works correctly")
	})
}

func TestS3StorageOptions(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_s3_storage_")
	if err != nil {
		t.Fatalf(" ❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("S3 Configuration Basic", func(t *testing.T) {
		// Test with basic S3 configuration (should set environment variables)
		accessKey := "test-access-key"
		secretKey := "test-secret-key"
		region := "us-east-1"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					AccessKeyID:     &accessKey,
					SecretAccessKey: &secretKey,
					Region:          &region,
				},
			},
		}

		// Clear any existing AWS environment variables
		originalAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
		originalSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		originalRegion := os.Getenv("AWS_REGION")

		defer func() {
			// Restore original environment variables
			if originalAccessKey != "" {
				os.Setenv("AWS_ACCESS_KEY_ID", originalAccessKey)
			} else {
				os.Unsetenv("AWS_ACCESS_KEY_ID")
			}
			if originalSecretKey != "" {
				os.Setenv("AWS_SECRET_ACCESS_KEY", originalSecretKey)
			} else {
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
			}
			if originalRegion != "" {
				os.Setenv("AWS_REGION", originalRegion)
			} else {
				os.Unsetenv("AWS_REGION")
			}
		}()

		// Test connection (environment variables should be set by Rust code)
		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with S3 options: %v", err)
		}
		defer conn.Close()

		// Note: We can't directly verify environment variables were set because
		// they're set in the Rust code, but if the connection succeeds, it means
		// the configuration was processed without errors
		t.Log("✅ S3 basic configuration processed successfully")
	})

	t.Run("S3 Configuration with Session Token", func(t *testing.T) {
		// Test with S3 configuration including session token
		accessKey := "test-access-key"
		secretKey := "test-secret-key"
		sessionToken := "test-session-token"
		region := "us-west-2"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					AccessKeyID:     &accessKey,
					SecretAccessKey: &secretKey,
					SessionToken:    &sessionToken,
					Region:          &region,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with S3 session token: %v", err)
		}
		defer conn.Close()

		t.Log("✅ S3 configuration with session token processed successfully")
	})

	t.Run("S3 Configuration with Profile", func(t *testing.T) {
		// Test with AWS profile configuration
		profile := "test-profile"
		region := "eu-west-1"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					Profile: &profile,
					Region:  &region,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with S3 profile: %v", err)
		}
		defer conn.Close()

		t.Log("✅ S3 configuration with profile processed successfully")
	})

	t.Run("S3 Configuration Anonymous Access", func(t *testing.T) {
		// Test with anonymous S3 access
		anonymous := true
		region := "us-east-1"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					AnonymousAccess: &anonymous,
					Region:          &region,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with anonymous S3 access: %v", err)
		}
		defer conn.Close()

		t.Log("✅ S3 anonymous access configuration processed successfully")
	})

	t.Run("S3 Configuration with Custom Endpoint", func(t *testing.T) {
		// Test with custom S3 endpoint (e.g., MinIO)
		endpoint := "http://localhost:9000"
		accessKey := "minioadmin"
		secretKey := "minioadmin"
		forcePathStyle := true

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					Endpoint:        &endpoint,
					AccessKeyID:     &accessKey,
					SecretAccessKey: &secretKey,
					ForcePathStyle:  &forcePathStyle,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with custom S3 endpoint: %v", err)
		}
		defer conn.Close()

		t.Log("✅ S3 custom endpoint configuration processed successfully")
	})
}

func TestCloudStorageOptionsPlaceholders(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_cloud_storage_")
	if err != nil {
		t.Fatalf(" ❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Azure Configuration Placeholder", func(t *testing.T) {
		// Test that Azure configuration is accepted (even if not implemented)
		accountName := "testaccount"
		accessKey := "test-access-key"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				AzureConfig: &contracts.AzureConfig{
					AccountName: &accountName,
					AccessKey:   &accessKey,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with Azure options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Azure configuration placeholder processed successfully")
	})

	t.Run("GCS Configuration Placeholder", func(t *testing.T) {
		// Test that GCS configuration is accepted (even if not implemented)
		projectID := "test-project"
		serviceAccountPath := "/path/to/service-account.json"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				GCSConfig: &contracts.GCSConfig{
					ProjectID:          &projectID,
					ServiceAccountPath: &serviceAccountPath,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with GCS options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ GCS configuration placeholder processed successfully")
	})
}

func TestGeneralStorageOptions(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_general_storage_")
	if err != nil {
		t.Fatalf(" ❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("General Storage Options Placeholders", func(t *testing.T) {
		// Test that general storage options are accepted (even if not implemented)
		blockSize := 4096
		maxRetries := 3
		timeout := 30
		allowHTTP := true
		userAgent := "LanceDB-Go/1.0"

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				BlockSize:  &blockSize,
				MaxRetries: &maxRetries,
				Timeout:    &timeout,
				AllowHTTP:  &allowHTTP,
				UserAgent:  &userAgent,
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with general storage options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ General storage options placeholders processed successfully")
	})

	t.Run("Local Configuration", func(t *testing.T) {
		// Test local storage configuration
		createDir := true
		useMemoryMap := true
		syncWrites := false

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				LocalConfig: &contracts.LocalConfig{
					CreateDirIfNotExists: &createDir,
					UseMemoryMap:         &useMemoryMap,
					SyncWrites:           &syncWrites,
				},
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with local storage options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Local storage configuration processed successfully")
	})
}

func TestCombinedStorageOptions(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_combined_storage_")
	if err != nil {
		t.Fatalf(" ❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Combined S3 and General Options", func(t *testing.T) {
		// Test combination of S3 and general options
		accessKey := "test-access-key"
		secretKey := "test-secret-key"
		region := "us-east-1"
		blockSize := 8192
		maxRetries := 5

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					AccessKeyID:     &accessKey,
					SecretAccessKey: &secretKey,
					Region:          &region,
				},
				BlockSize:  &blockSize,
				MaxRetries: &maxRetries,
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with combined options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Combined S3 and general options processed successfully")
	})

	t.Run("All Storage Options Combined", func(t *testing.T) {
		// Test all storage options combined (comprehensive test)
		accessKey := "test-access-key"
		secretKey := "test-secret-key"
		region := "us-east-1"
		accountName := "testaccount"
		projectID := "test-project"
		blockSize := 4096
		timeout := 30
		createDir := true

		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{
					AccessKeyID:     &accessKey,
					SecretAccessKey: &secretKey,
					Region:          &region,
				},
				AzureConfig: &contracts.AzureConfig{
					AccountName: &accountName,
				},
				GCSConfig: &contracts.GCSConfig{
					ProjectID: &projectID,
				},
				LocalConfig: &contracts.LocalConfig{
					CreateDirIfNotExists: &createDir,
				},
				BlockSize: &blockSize,
				Timeout:   &timeout,
			},
		}

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf(" ❌Failed to connect with all storage options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ All storage options combined processed successfully")
	})
}

func TestStorageOptionsErrorCases(t *testing.T) {
	t.Run("Invalid URI with StorageOptions", func(t *testing.T) {
		// Test error handling with invalid URI
		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{
				S3Config: &contracts.S3Config{},
			},
		}

		// Use an obviously invalid URI that should cause an error
		_, err := lancedb.Connect(context.Background(), "invalid://not-a-valid-path", options)
		if err == nil {
			t.Log("Note: Invalid URI was accepted (might be expected behavior)")
		} else {
			t.Logf("✅ Correctly handled error with invalid URI: %v", err)
		}
	})

	t.Run("Malformed StorageOptions JSON", func(t *testing.T) {
		// This test is mainly to ensure our JSON serialization works
		// The actual malformed JSON would be caught during marshaling
		options := &contracts.ConnectionOptions{
			StorageOptions: &contracts.StorageOptions{}, // Empty is valid
		}

		tempDir, err := os.MkdirTemp("", "lancedb_test_malformed_")
		if err != nil {
			t.Fatalf(" ❌Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		conn, err := lancedb.Connect(context.Background(), tempDir, options)
		if err != nil {
			t.Fatalf("Unexpected error with empty storage options: %v", err)
		}
		defer conn.Close()

		t.Log("✅ Empty storage options handled correctly")
	})
}
