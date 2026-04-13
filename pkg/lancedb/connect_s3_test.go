//go:build integration

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
	minioBucket = "test-bucket"
	minioRegion = "us-east-1"
)

// Package-level state set by TestMain.
var minioEndpoint string

func minioOptions() *contracts.ConnectionOptions {
	return &contracts.ConnectionOptions{
		StorageOptions: map[string]string{
			contracts.StorageAccessKeyID:               "minioadmin",
			contracts.StorageSecretAccessKey:           "minioadmin",
			contracts.StorageEndpoint:                  minioEndpoint,
			contracts.StorageRegion:                    minioRegion,
			contracts.StorageAllowHTTP:                 "true",
			contracts.StorageVirtualHostedStyleRequest: "false",
		},
	}
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start MinIO via testcontainers
	container, err := tcminio.Run(ctx, "minio/minio:latest",
		tcminio.WithUsername("minioadmin"),
		tcminio.WithPassword("minioadmin"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start MinIO container: %v\n", err)
		os.Exit(1)
	}

	// Get the dynamic endpoint
	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get MinIO connection string: %v\n", err)
		os.Exit(1)
	}
	minioEndpoint = "http://" + connStr

	// Create the test bucket via MinIO client
	mc, err := miniogo.New(connStr, &miniogo.Options{
		Creds:  credentials.NewStaticV4(container.Username, container.Password, ""),
		Secure: false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create MinIO client: %v\n", err)
		os.Exit(1)
	}

	if err := mc.MakeBucket(ctx, minioBucket, miniogo.MakeBucketOptions{Region: minioRegion}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create bucket: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if err := container.Terminate(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate container: %v\n", err)
	}

	os.Exit(code)
}

func TestConnectMinIO(t *testing.T) {
	ctx := context.Background()
	db, err := lancedb.Connect(ctx, fmt.Sprintf("s3://%s/connect-test", minioBucket), minioOptions())
	if err != nil {
		t.Fatalf("Failed to connect to MinIO: %v", err)
	}
	defer db.Close()

	if db.IsClosed() {
		t.Fatal("Connection should not be closed")
	}
}

func TestMinIOCreateAndOpenTable(t *testing.T) {
	ctx := context.Background()
	db, err := lancedb.Connect(ctx, fmt.Sprintf("s3://%s/crud-test", minioBucket), minioOptions())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "vector", Type: arrow.FixedSizeListOf(4, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create table
	table, err := db.CreateTable(ctx, "test_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Insert data
	pool := memory.NewGoAllocator()

	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	textBuilder := array.NewStringBuilder(pool)
	textBuilder.AppendValues([]string{"hello", "world", "test"}, nil)
	textArray := textBuilder.NewArray()
	defer textArray.Release()

	vectorBuilder := array.NewFloat32Builder(pool)
	vectorBuilder.AppendValues([]float32{
		0.1, 0.2, 0.3, 0.4,
		0.5, 0.6, 0.7, 0.8,
		0.9, 1.0, 1.1, 1.2,
	}, nil)
	vectorFloat32 := vectorBuilder.NewArray()
	defer vectorFloat32.Release()

	vectorListType := arrow.FixedSizeListOf(4, arrow.PrimitiveTypes.Float32)
	vectorArray := array.NewFixedSizeListData(
		array.NewData(vectorListType, 3, []*memory.Buffer{nil},
			[]arrow.ArrayData{vectorFloat32.Data()}, 0, 0),
	)
	defer vectorArray.Release()

	record := array.NewRecord(arrowSchema, []arrow.Array{idArray, textArray, vectorArray}, 3)
	defer record.Release()

	if err := table.Add(ctx, record, nil); err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Verify count
	count, err := table.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}
	if count != 3 {
		t.Fatalf("Expected 3 rows, got %d", count)
	}

	// Open existing table
	table2, err := db.OpenTable(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to open table: %v", err)
	}
	defer table2.Close()

	count2, err := table2.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count reopened table: %v", err)
	}
	if count2 != 3 {
		t.Fatalf("Expected 3 rows after reopen, got %d", count2)
	}
}

func TestMinIOConcurrentConnections(t *testing.T) {
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			path := fmt.Sprintf("s3://%s/concurrent-test-%d", minioBucket, idx)
			db, err := lancedb.Connect(ctx, path, minioOptions())
			if err != nil {
				errs <- fmt.Errorf("goroutine %d connect failed: %w", idx, err)
				return
			}
			defer db.Close()

			fields := []arrow.Field{
				{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			}
			arrowSchema := arrow.NewSchema(fields, nil)
			schema, err := lancedb.NewSchema(arrowSchema)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d schema failed: %w", idx, err)
				return
			}

			tableName := fmt.Sprintf("table_%d", idx)
			tbl, err := db.CreateTable(ctx, tableName, schema)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d create table failed: %w", idx, err)
				return
			}
			defer tbl.Close()
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
}

func TestMinIOInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	opts := &contracts.ConnectionOptions{
		StorageOptions: map[string]string{
			contracts.StorageAccessKeyID:               "wrong-key",
			contracts.StorageSecretAccessKey:           "wrong-secret",
			contracts.StorageEndpoint:                  minioEndpoint,
			contracts.StorageRegion:                    minioRegion,
			contracts.StorageAllowHTTP:                 "true",
			contracts.StorageVirtualHostedStyleRequest: "false",
		},
	}

	db, err := lancedb.Connect(ctx, fmt.Sprintf("s3://%s/bad-creds", minioBucket), opts)
	if err != nil {
		// Connection itself might fail — that's fine
		return
	}
	defer db.Close()

	// If connect succeeded, operations should fail
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Schema creation should not fail: %v", err)
	}

	_, err = db.CreateTable(ctx, "should_fail", schema)
	if err == nil {
		t.Fatal("Expected error with invalid credentials")
	}
}

func TestMinIOInvalidEndpoint(t *testing.T) {
	ctx := context.Background()
	opts := &contracts.ConnectionOptions{
		StorageOptions: map[string]string{
			contracts.StorageAccessKeyID:               "minioadmin",
			contracts.StorageSecretAccessKey:           "minioadmin",
			contracts.StorageEndpoint:                  "http://localhost:19999",
			contracts.StorageRegion:                    minioRegion,
			contracts.StorageAllowHTTP:                 "true",
			contracts.StorageVirtualHostedStyleRequest: "false",
		},
	}

	db, err := lancedb.Connect(ctx, "s3://nonexistent-bucket/test", opts)
	if err != nil {
		// Connection failure is acceptable
		return
	}
	defer db.Close()

	// If connect succeeded, operations should fail
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Schema creation should not fail: %v", err)
	}

	_, err = db.CreateTable(ctx, "should_fail", schema)
	if err == nil {
		t.Fatal("Expected error with invalid endpoint")
	}
}
