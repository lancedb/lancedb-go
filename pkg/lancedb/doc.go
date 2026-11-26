// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

/*
Package lancedb provides Go bindings for LanceDB, an open-source vector database.

The package is pure Go. It builds with CGO_ENABLED=0 and loads the published
lancedb-c shared library at runtime through purego, so consumers need no Rust
toolchain, no C compiler and no cgo build. The native library is delivered
through compiled-in companion modules and is never downloaded at runtime.

# Requirements

  - Go 1.21 or newer.
  - CGO_ENABLED=0.
  - Linux amd64/arm64, Windows amd64/arm64 or macOS on Apple Silicon
    (darwin/arm64). Intel macOS is not supported.
  - Apache Arrow Go v17 types (github.com/apache/arrow/go/v17).
  - glibc 2.35 or newer for the published Linux libraries.

# Installing

Preview releases are published on the releases page
(https://github.com/lancedb/lancedb-go/releases). Choose a preview from that
page whose release notes specify module-backed offline native loading, then use
that same tag in both commands below (replacing vX.Y.Z-preview.N):

	go get github.com/lancedb/lancedb-go@vX.Y.Z-preview.N

@latest selects the highest release version when the module has one; only a
module with no release version falls back to a prerelease or a pseudo-version.
The stable v0.1.2 release predates the pure-Go preview and does not contain the
native payload or the lancedb-go-native helper, so name a preview explicitly.
The root module requires one companion artifact module per target
(go.lancedb.io/lancedb-go/artifacts/<goos>-<goarch>); build-tagged selectors
link exactly the module for the target being built.

To pre-seed the native cache, for example in an offline image or CI, run the
helper from the same tag:

	go run github.com/lancedb/lancedb-go/pkg/cmd/lancedb-go-native@vX.Y.Z-preview.N

# Native Library Loading

The first operation that needs the native library verifies the compiled-in
payload for the running target, extracts it into a private per-user cache and
loads it. When LANCEDB_C_LIBRARY_PATH is set, that explicit file replaces the
compiled-in payload. Neither path downloads anything at runtime, so the SDK
works offline once go get has resolved the companion modules. No source build
happens on the consumer machine.

Discovery order:

  - LANCEDB_C_LIBRARY_PATH, when set, must name an existing shared-library
    file. It is then the only candidate; an invalid value fails instead of
    falling back to another library.
  - The compiled-in payload for the running target, materialized in the
    private cache.
  - The platform library name resolved by the system dynamic loader. The loader
    tries the system library only when installing the compiled-in payload
    fails; if the verified cache file itself cannot be loaded, the error is
    returned without a system-loader fallback.

LANCEDB_C_CACHE_DIR overrides the cache root. The default is
os.UserCacheDir()/lancedb-go/native/<lancedb-c version>/<target>.

The loaded library must report exactly the lancedb-c version this Go module was
built against. A different version fails before use because the binding's C
header and ABI must match. RuntimeVersion reports the loaded version. The
library is loaded at most once per process and is never unloaded: lancedb-c
owns native threads and handles that may outlive a table.

# Basic Usage

	ctx := context.Background()
	db, err := lancedb.Connect(ctx, "./my_database", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddVectorField("embedding", 128, contracts.VectorDataTypeFloat32, false).
		AddStringField("text", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	table, err := db.CreateTable(ctx, "documents", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

# Key Features

  - Vector search: similarity search over dense vectors with L2, cosine, dot
    product and Hamming distance.
  - Multi-modal data: vectors, metadata, text, images and other values in one
    table.
  - SQL-style filters: predicates use the LanceDB SQL dialect, where double
    quotes delimit identifiers and single quotes delimit string literals.
  - Multiple backends: local file systems, S3 and S3-compatible storage,
    Google Cloud Storage and Azure Blob Storage.
  - Scalable indexing: IVF-PQ, IVF-Flat, HNSW-PQ, HNSW-SQ, BTree, Bitmap,
    label-list and full-text indexes.
  - Versioned datasets: immutable table versions with checkout and tags.
  - Arrow-native: schemas, input records and results use Apache Arrow Go v17
    and cross the native boundary as Arrow IPC.

# Vector Search

	queryVector := []float32{0.1, 0.2, 0.3}
	results, err := table.VectorSearch(ctx, "embedding", queryVector, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Vector search with a filter.
	filtered, err := table.VectorSearchWithFilter(ctx, "embedding", queryVector, 5, "text IS NOT NULL")
	if err != nil {
		log.Fatal(err)
	}

# Cloud Storage

Connect preserves the published v0.1.2 ConnectionOptions shape. Its structured
S3Config only forwards AccessKeyID, SecretAccessKey, SessionToken, Region and
Profile; top-level Region and ReadConsistencyInterval remain ignored, as they
were in v0.1.2. Use ConnectWithStorageOptions for arbitrary native storage
options:

	db, err := lancedb.ConnectWithStorageOptions(ctx, "s3://my-bucket/db-prefix", map[string]string{
		contracts.StorageAccessKeyID:     "your-key",
		contracts.StorageSecretAccessKey: "your-secret",
		contracts.StorageAWSRegion:       "us-west-2",
	})

Options use the native lancedb-c keys exposed as contracts.Storage* constants.
For example, a local MinIO endpoint:

	db, err := lancedb.ConnectWithStorageOptions(ctx, "s3://my-bucket/db-prefix", map[string]string{
		contracts.StorageEndpoint:          "http://127.0.0.1:9000",
		contracts.StorageAllowHTTP:         "true",
		contracts.StorageS3AddressingStyle: "path",
	})

Azure Blob Storage:

	db, err := lancedb.ConnectWithStorageOptions(ctx, "az://container/prefix", map[string]string{
		contracts.StorageAzureAccountName: "your-account",
		contracts.StorageAzureAccessKey:   "your-key",
	})

# Schema Building

Build schemas with a fluent interface. Build returns contracts.ISchema:

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).                                              // Required integer
		AddVectorField("embedding", 384, contracts.VectorDataTypeFloat32, false). // 384-dimension vector
		AddStringField("text", true).                                            // Optional string
		AddFloat32Field("score", true).                                          // Optional float
		AddTimestampField("created_at", arrow.Microsecond, true).                // Optional timestamp
		AddBooleanField("active", true).                                         // Optional boolean
		AddBinaryField("metadata", true).                                        // Optional binary data
		Build()

# Adding Data

Records cross the boundary as Arrow IPC. Build records with the table schema:

	recordBuilder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer recordBuilder.Release()
	// Append values to recordBuilder.Field(i) ...
	record := recordBuilder.NewRecord()
	defer record.Release()

	err = table.AddRecords(ctx, []arrow.Record{record}, nil)

Add is retained for v0.1.2 compatibility and accepts a single record. Prefer
AddRecords for batches.

# Querying

The published single-call helpers are available directly on ITable:

	// Limit with an offset.
	rows, err := table.SelectWithLimit(ctx, 100, 0)

	// Filter and projection.
	rows, err = table.SelectWithFilter(ctx, "score > 0.8")
	rows, err = table.SelectWithColumns(ctx, []string{"id", "text", "score"})

	// Full QueryConfig.
	limit := 100
	rows, err = table.Select(ctx, contracts.QueryConfig{
		Where: "score > 0.8",
		Limit: &limit,
	})

Full-text and hybrid search use the additive query interfaces. The published
FullTextSearch and FullTextSearchWithFilter methods remain for v0.1.2
compatibility and return the legacy unsupported error. Use FTSSearch with
SelectV2, or WithFullText on a vector query for hybrid search.

# Compatibility and Companion Interfaces

The published v0.1.2 API is preserved: IConnection, ITable, IQueryBuilder,
IVectorQueryBuilder, ISchema, ISchemaBuilder and the configuration types keep
their v0.1.2 method sets and field layouts. In particular,
IQueryBuilder.Execute returns ([]arrow.Record, error).

Additive capabilities use companion interfaces with separate entry points.
Assert the optional interface on a table before using it:

	qv2, ok := table.(contracts.ITableQueryV2)
	if !ok {
		return fmt.Errorf("table does not implement ITableQueryV2")
	}
	record, err := qv2.QueryV2().
		Filter("score > 0.8").
		Columns([]string{"id", "text"}).
		Limit(10).
		Execute(ctx)

ITableVectorQuery provides vector query builders, while
ITableIndexManagement, ITableMergeInsert, ITableOptimize, ITableUpdateExpr,
ITableTimeTravel, ITableSchemaEvolve and ITablePrewarmIndex expose the other
additive capabilities. Package contracts documents the compatibility boundary
and the full list of additive types.

# Index Management

Create and manage indexes for better query performance. Every index method
takes a context:

	// Create a vector index.
	err = table.CreateIndex(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq)

	// Create a named index.
	err = table.CreateIndexWithName(ctx, []string{"text"}, contracts.IndexTypeFts, "text_search_idx")

	// Scalar indexes.
	err = table.CreateIndex(ctx, []string{"id"}, contracts.IndexTypeBTree)
	err = table.CreateIndex(ctx, []string{"category"}, contracts.IndexTypeBitmap)

	// List all indexes.
	indexes, err := table.GetAllIndexes(ctx)
	for _, idx := range indexes {
		fmt.Printf("Index: %s, Columns: %v, Type: %s\n", idx.Name, idx.Columns, idx.IndexType)
	}

Available index types:

	contracts.IndexTypeAuto      // Auto-select the best index type
	contracts.IndexTypeIvfPq     // IVF-PQ for large vector datasets
	contracts.IndexTypeIvfFlat   // IVF-Flat for exact vector search
	contracts.IndexTypeHnswPq    // HNSW-PQ for high-performance vector search
	contracts.IndexTypeHnswSq    // HNSW-SQ for scalar-quantized vectors
	contracts.IndexTypeBTree     // BTree for scalar fields
	contracts.IndexTypeBitmap    // Bitmap for low-cardinality fields
	contracts.IndexTypeLabelList // Label list for multi-label fields
	contracts.IndexTypeFts       // Full-text search index

ITableIndexManagement adds parameterized creation, index statistics,
WaitForIndex and DropIndex. ITablePrewarmIndex adds PrewarmIndex.

# Table Operations

	// Table information.
	name := table.Name()
	count, err := table.Count(ctx)
	version, err := table.Version(ctx)
	schema, err := table.Schema(ctx)

	// Update records with JSON scalar values.
	updates := map[string]interface{}{
		"score":      0.95,
		"updated_at": time.Now(),
	}
	err = table.Update(ctx, "id = 123", updates)

	// Delete records.
	err = table.Delete(ctx, "score < 0.1")

# Error Handling

Standard Go error handling patterns are used throughout the SDK:

	if err != nil {
		// Handle the error appropriately.
		log.Printf("Operation failed: %v", err)
		return err
	}

# Performance Considerations

  - Use AddRecords to amortize per-call overhead when inserting many rows.
  - Create indexes that match the query patterns and data distribution.
  - Choose vector search k values that balance latency and recall.
  - Apply filters to reduce the candidate set.
  - Prefer projections over selecting every column from wide tables.

# Thread Safety

Connection and Table objects are safe for concurrent use. Individual query
builders are not: do not share a builder between goroutines.

# Memory Management

Call Close on connections and tables when done, and release Arrow records when
they are no longer needed. The SDK closes native handles automatically when
objects are finalized, but explicit Close keeps resource lifetime
deterministic.

# Documentation

  - Go package reference: https://pkg.go.dev/github.com/lancedb/lancedb-go/pkg/lancedb
  - Contracts reference: https://pkg.go.dev/github.com/lancedb/lancedb-go/pkg/contracts
  - Migration guide: https://github.com/lancedb/lancedb-go/blob/main/MIGRATION.md
  - Repository: https://github.com/lancedb/lancedb-go
  - LanceDB documentation: https://lancedb.github.io/lancedb/
*/
package lancedb
