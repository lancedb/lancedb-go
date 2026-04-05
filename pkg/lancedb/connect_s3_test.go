//go:build integration

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const (
	minioEndpoint  = "http://localhost:9000"
	minioAccessKey = "minioadmin"
	minioSecretKey = "minioadmin"
	minioBucket    = "test-bucket"
	minioRegion    = "us-east-1"
)

func minioOptions() *contracts.ConnectionOptions {
	return &contracts.ConnectionOptions{
		StorageOptions: map[string]string{
			contracts.StorageAccessKeyID:               minioAccessKey,
			contracts.StorageSecretAccessKey:           minioSecretKey,
			contracts.StorageEndpoint:                  minioEndpoint,
			contracts.StorageRegion:                    minioRegion,
			contracts.StorageAllowHTTP:                 "true",
			contracts.StorageVirtualHostedStyleRequest: "false",
		},
	}
}

func TestMain(m *testing.M) {
	// Wait for MinIO to be ready (bucket is created by the createbucket
	// sidecar in docker-compose.yml)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, minioEndpoint+"/minio/health/live", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if ctx.Err() != nil {
			fmt.Fprintf(os.Stderr, "MinIO not ready after 30s: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for bucket to exist (created by docker-compose sidecar)
	bucketCtx, bucketCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer bucketCancel()
	for {
		req, _ := http.NewRequestWithContext(bucketCtx, http.MethodHead, minioEndpoint+"/"+minioBucket, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if bucketCtx.Err() != nil {
			fmt.Fprintf(os.Stderr, "Bucket %s not ready after 30s\n", minioBucket)
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}

	os.Exit(m.Run())
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
			contracts.StorageAccessKeyID:               minioAccessKey,
			contracts.StorageSecretAccessKey:           minioSecretKey,
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
