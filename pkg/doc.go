// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

/*
Package lancedb provides Go bindings for LanceDB, an open-source vector database.

LanceDB is designed for AI applications that need to store, manage, and query
high-dimensional vector embeddings alongside traditional data types. This Go SDK
provides a comprehensive interface to all LanceDB features through CGO bindings
to the Rust core library.

# Key Features

• Vector Search: High-performance similarity search with multiple distance metrics (L2, cosine, dot product)
• Multi-modal Data: Store vectors, metadata, text, images, and more in a single database
• SQL Queries: Query your data using familiar SQL syntax via DataFusion integration
• Multiple Backends: Local filesystem, S3, Google Cloud Storage, and Azure support
• Scalable Indexing: Support for IVF-PQ, IVF-Flat, HNSW-PQ, BTree, Bitmap, and FTS indexes
• ACID Transactions: Full transactional support with automatic versioning
• Zero-Copy Operations: Efficient memory usage through Apache Arrow integration

# Basic Usage

Connect to a database and perform basic operations:

	db, err := lancedb.Connect(context.Background(), "./my_database", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create schema
	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddVectorField("embedding", 128, lancedb.VectorDataTypeFloat32, false).
		AddStringField("text", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Create table
	table, err := db.CreateTable(context.Background(), "documents", *schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

# Vector Search

Perform similarity search on vector embeddings:

	queryVector := []float32{0.1, 0.2, 0.3}
	results, err := table.VectorSearch("embedding", queryVector, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Vector search with filtering
	filteredResults, err := table.VectorSearchWithFilter("embedding", queryVector, 5, "text IS NOT NULL")
	if err != nil {
		log.Fatal(err)
	}

# Connection Types

The Connect function supports multiple storage backends through URI schemes:

Local database:

	db, err := lancedb.Connect(context.Background(), "/path/to/database", nil)

S3-based database:

	opts := &lancedb.ConnectionOptions{
		StorageOptions: &lancedb.StorageOptions{
			S3Config: &lancedb.S3Config{
				Region:          "us-west-2",
				AccessKeyId:     "your-key",
				SecretAccessKey: "your-secret",
			},
		},
	}
	db, err := lancedb.Connect(context.Background(), "s3://my-bucket/db-prefix", opts)

Azure Storage:

	opts := &lancedb.ConnectionOptions{
		StorageOptions: &lancedb.StorageOptions{
			AzureConfig: &lancedb.AzureConfig{
				AccountName: "your-account",
				AccessKey:   "your-key",
			},
		},
	}
	db, err := lancedb.Connect(context.Background(), "az://container/prefix", opts)

# Schema Building

Build schemas with a fluent interface:

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).                                    // Required integer
		AddVectorField("embedding", 384, lancedb.VectorDataTypeFloat32, false). // 384-dim vector
		AddStringField("text", true).                                  // Optional string
		AddFloat32Field("score", true).                               // Optional float
		AddTimestampField("created_at", arrow.Microsecond, true).     // Optional timestamp
		AddBooleanField("active", true).                              // Optional boolean
		AddBinaryField("metadata", true).                             // Optional binary data
		Build()

# Adding Data

Add records to tables using Apache Arrow records:

	// Create sample data as Arrow record
	pool := memory.NewGoAllocator()

	// Build the record with your data
	record := // ... create arrow.Record with your data

	// Add single record
	err = table.Add(record, nil)

	// Add multiple records
	records := []arrow.Record{record1, record2, record3}
	err = table.AddRecords(records, nil)

# Query Operations

Various query operations available:

	// Basic select with limit
	results, err := table.SelectWithLimit(100, 0)

	// Select with filter
	results, err := table.SelectWithFilter("score > 0.8")

	// Select specific columns
	results, err := table.SelectWithColumns([]string{"id", "text", "score"})

	// Full-text search
	results, err := table.FullTextSearch("text", "search query")

	// Full-text search with filter
	results, err := table.FullTextSearchWithFilter("text", "search query", "score > 0.5")

# Index Management

Create and manage indexes for better query performance:

	// Create a vector index
	err = table.CreateIndex([]string{"embedding"}, lancedb.IndexTypeIvfPq)

	// Create a named index
	err = table.CreateIndexWithName([]string{"text"}, lancedb.IndexTypeFts, "text_search_idx")

	// Create other index types
	err = table.CreateIndex([]string{"id"}, lancedb.IndexTypeBTree)      // BTree for scalars
	err = table.CreateIndex([]string{"category"}, lancedb.IndexTypeBitmap) // Bitmap for low cardinality

	// List all indexes
	indexes, err := table.GetAllIndexes()
	for _, idx := range indexes {
		fmt.Printf("Index: %s, Columns: %v, Type: %s\n", idx.Name, idx.Columns, idx.IndexType)
	}

# Available Index Types

	lancedb.IndexTypeAuto        // Auto-select best index type
	lancedb.IndexTypeIvfPq       // IVF-PQ for large vector datasets
	lancedb.IndexTypeIvfFlat     // IVF-Flat for exact vector search
	lancedb.IndexTypeHnswPq      // HNSW-PQ for high-performance vector search
	lancedb.IndexTypeHnswSq      // HNSW-SQ for scalar quantized vectors
	lancedb.IndexTypeBTree       // BTree for scalar fields
	lancedb.IndexTypeBitmap      // Bitmap for low-cardinality fields
	lancedb.IndexTypeLabelList   // Label list for multi-label fields
	lancedb.IndexTypeFts         // Full-text search index

# Table Operations

	// Get table information
	name := table.Name()
	count, err := table.Count()
	version, err := table.Version()
	schema, err := table.Schema()

	// Update records
	updates := map[string]interface{}{
		"score": 0.95,
		"updated_at": time.Now(),
	}
	err = table.Update("id = 123", updates)

	// Delete records
	err = table.Delete("score < 0.1")

# Error Handling

Standard Go error handling patterns are used throughout the SDK:

	if err != nil {
		// Handle error appropriately
		log.Printf("Operation failed: %v", err)
		return err
	}

# Performance Considerations

• Use batch operations when inserting large amounts of data via AddRecords()
• Create appropriate indexes for your query patterns
• Use vector search with appropriate k values to balance speed and recall
• Leverage Arrow's zero-copy operations when possible
• Consider using filters to reduce search space

# Thread Safety

Connection and Table objects are thread-safe and can be used concurrently
from multiple goroutines. However, individual query builders are not thread-safe
and should not be shared between goroutines.

# Memory Management

The SDK handles memory management automatically. Make sure to:
• Call Close() on connections and tables when done
• Release Arrow records when appropriate to free memory

For more detailed examples and advanced usage, see the examples directory
and the full documentation at https://lancedb.github.io/lancedb/
*/
package lancedb
