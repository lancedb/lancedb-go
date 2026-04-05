// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Storage Configuration Example
//
// This example demonstrates storage configuration capabilities with LanceDB
// using the Go SDK. It covers:
// - Local file system storage
// - AWS S3 storage configuration
// - MinIO object storage for local development
// - Cloudflare R2 configuration
// - Google Cloud Storage configuration
// - Azure Blob Storage configuration

package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	. "github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const (
	VectorDim     = 128
	SampleRecords = 100
)

func main() {
	fmt.Println("LanceDB Go SDK - Storage Configuration Example")
	fmt.Println("================================================")

	ctx := context.Background()

	// Local storage
	fmt.Println("\nStep 1: Local storage...")
	if err := demonstrateLocalStorage(ctx); err != nil {
		log.Printf("Local storage demo failed: %v", err)
	}

	// Cloud storage configurations (display only - require real credentials)
	fmt.Println("\nStep 2: Cloud storage configurations...")
	showCloudConfigurations()

	fmt.Println("\nStorage configuration examples completed!")
}

func demonstrateLocalStorage(ctx context.Context) error {
	// Basic local storage (no options needed)
	tempDir, err := os.MkdirTemp("", "lancedb_local_")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(ctx, tempDir, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	fmt.Printf("  Local storage at: %s\n", tempDir)

	// Test basic operations
	if err := testStorageConfiguration("Local", conn, ctx); err != nil {
		return fmt.Errorf("local storage test failed: %w", err)
	}

	return nil
}

func showCloudConfigurations() {
	// AWS S3
	fmt.Println("  AWS S3:")
	s3Opts := &ConnectionOptions{
		StorageOptions: map[string]string{
			StorageAccessKeyID:     "AKIA...",
			StorageSecretAccessKey: "wJalr...",
			StorageRegion:          "us-east-1",
		},
	}
	printConfig("S3", s3Opts.StorageOptions)

	// MinIO
	fmt.Println("\n  MinIO (S3-compatible):")
	minioOpts := &ConnectionOptions{
		StorageOptions: map[string]string{
			StorageAccessKeyID:               "minioadmin",
			StorageSecretAccessKey:           "minioadmin",
			StorageEndpoint:                  "http://localhost:9000",
			StorageRegion:                    "us-east-1",
			StorageAllowHTTP:                 "true",
			StorageVirtualHostedStyleRequest: "false",
		},
	}
	printConfig("MinIO", minioOpts.StorageOptions)

	// Cloudflare R2
	fmt.Println("\n  Cloudflare R2:")
	r2Opts := &ConnectionOptions{
		StorageOptions: map[string]string{
			StorageAccessKeyID:     "...",
			StorageSecretAccessKey: "...",
			StorageAWSEndpoint:     "https://ACCOUNT_ID.r2.cloudflarestorage.com",
			StorageRegion:          "auto",
		},
	}
	printConfig("R2", r2Opts.StorageOptions)
	fmt.Println("    Note: R2 requires StorageAWSEndpoint (not StorageEndpoint)")

	// Google Cloud Storage
	fmt.Println("\n  Google Cloud Storage:")
	gcsOpts := &ConnectionOptions{
		StorageOptions: map[string]string{
			StorageGCSServiceAccount: "/path/to/service-account.json",
		},
	}
	printConfig("GCS", gcsOpts.StorageOptions)

	// Azure Blob Storage
	fmt.Println("\n  Azure Blob Storage:")
	azureOpts := &ConnectionOptions{
		StorageOptions: map[string]string{
			StorageAzureAccountName: "myaccount",
			StorageAzureAccessKey:   "...",
		},
	}
	printConfig("Azure", azureOpts.StorageOptions)
}

func printConfig(name string, opts map[string]string) {
	fmt.Printf("    %s configuration (%d keys):\n", name, len(opts))
	for k, v := range opts {
		// Mask sensitive values
		display := v
		if len(v) > 12 {
			display = v[:8] + "..."
		}
		fmt.Printf("      %s = %s\n", k, display)
	}
}

// Helper functions

func testStorageConfiguration(name string, conn IConnection, ctx context.Context) error {
	table, schema, err := createTestTable(conn, ctx, fmt.Sprintf("test_%s", name))
	if err != nil {
		return err
	}
	defer table.Close()

	testData := generateTestData(SampleRecords)
	if err := insertTestData(table, schema, testData); err != nil {
		return err
	}

	queryVector := generateRandomVector(VectorDim)
	results, err := table.VectorSearch(context.Background(), "vector", queryVector, 5)
	if err != nil {
		return err
	}

	fmt.Printf("  %s: %d records inserted, %d results from query\n",
		name, len(testData), len(results))
	return nil
}

func createTestTable(conn IConnection, ctx context.Context, tableName string) (ITable, *arrow.Schema, error) {
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "vector", Type: arrow.FixedSizeListOf(VectorDim, arrow.PrimitiveTypes.Float32), Nullable: false},
	}

	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		return nil, nil, err
	}

	table, err := conn.CreateTable(ctx, tableName, schema)
	if err != nil {
		return nil, nil, err
	}
	return table, arrowSchema, nil
}

func generateTestData(count int) []struct {
	ID     int32
	Name   string
	Vector []float32
} {
	records := make([]struct {
		ID     int32
		Name   string
		Vector []float32
	}, count)

	for i := 0; i < count; i++ {
		records[i].ID = int32(i + 1)
		records[i].Name = fmt.Sprintf("Record %d", i+1)
		records[i].Vector = generateRandomVector(VectorDim)
	}
	return records
}

func generateRandomVector(dimensions int) []float32 {
	vector := make([]float32, dimensions)
	for i := 0; i < dimensions; i++ {
		vector[i] = rand.Float32()*2 - 1
	}

	var norm float32
	for _, v := range vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}
	return vector
}

func insertTestData(table ITable, schema *arrow.Schema, data []struct {
	ID     int32
	Name   string
	Vector []float32
}) error {
	pool := memory.NewGoAllocator()

	ids := make([]int32, len(data))
	names := make([]string, len(data))
	allVectors := make([]float32, len(data)*VectorDim)

	for i, record := range data {
		ids[i] = record.ID
		names[i] = record.Name
		copy(allVectors[i*VectorDim:(i+1)*VectorDim], record.Vector)
	}

	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues(ids, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues(names, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	vectorBuilder := array.NewFloat32Builder(pool)
	vectorBuilder.AppendValues(allVectors, nil)
	vectorFloat32Array := vectorBuilder.NewArray()
	defer vectorFloat32Array.Release()

	vectorListType := arrow.FixedSizeListOf(VectorDim, arrow.PrimitiveTypes.Float32)
	vectorArray := array.NewFixedSizeListData(
		array.NewData(vectorListType, len(data), []*memory.Buffer{nil},
			[]arrow.ArrayData{vectorFloat32Array.Data()}, 0, 0),
	)
	defer vectorArray.Release()

	columns := []arrow.Array{idArray, nameArray, vectorArray}
	record := array.NewRecord(schema, columns, int64(len(data)))
	defer record.Release()

	return table.Add(context.Background(), record, nil)
}
