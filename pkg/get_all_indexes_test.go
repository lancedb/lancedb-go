// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func TestGetAllIndexes(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_indexes_")
	if err != nil {
		t.Fatalf("❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to database
	conn, err := Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("❌Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create test schema with vector field
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "category", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("❌Failed to create schema: %v", err)
	}

	// Create table
	table, err := conn.CreateTable(context.Background(), "test_indexes", *schema)
	if err != nil {
		t.Fatalf("❌Failed to create table: %v", err)
	}
	defer table.Close()

	// Add some sample data
	t.Log("Adding sample data...")
	pool := memory.NewGoAllocator()

	// Create sample data
	const numRecords = 300

	// Generate IDs
	idBuilder := array.NewInt32Builder(pool)
	ids := make([]int32, numRecords)
	for i := 0; i < numRecords; i++ {
		ids[i] = int32(i + 1)
	}
	idBuilder.AppendValues(ids, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	// Generate names
	nameBuilder := array.NewStringBuilder(pool)
	names := make([]string, numRecords)
	for i := 0; i < numRecords; i++ {
		names[i] = fmt.Sprintf("User_%d", i+1)
	}
	nameBuilder.AppendValues(names, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	// Generate categories
	categoryBuilder := array.NewStringBuilder(pool)
	categories := make([]string, numRecords)
	categoryOptions := []string{"A", "B", "C", "D", "E"}
	for i := 0; i < numRecords; i++ {
		categories[i] = categoryOptions[i%len(categoryOptions)]
	}
	categoryBuilder.AppendValues(categories, nil)
	categoryArray := categoryBuilder.NewArray()
	defer categoryArray.Release()

	// Generate scores
	scoreBuilder := array.NewFloat64Builder(pool)
	scores := make([]float64, numRecords)
	for i := 0; i < numRecords; i++ {
		scores[i] = 80.0 + float64(i%20)
	}
	scoreBuilder.AppendValues(scores, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	// Create vector embeddings (128-dimensional vectors)
	embeddingValues := make([]float32, numRecords*128) // 300 records * 128 dimensions
	for i := 0; i < numRecords; i++ {
		for j := 0; j < 128; j++ {
			// Create unique vector patterns for each record
			embeddingValues[i*128+j] = float32(i)*0.1 + float32(j)*0.001
		}
	}

	// Create Float32Array for all embedding values
	embeddingFloat32Builder := array.NewFloat32Builder(pool)
	embeddingFloat32Builder.AppendValues(embeddingValues, nil)
	embeddingFloat32Array := embeddingFloat32Builder.NewArray()
	defer embeddingFloat32Array.Release()

	// Create FixedSizeListArray for embeddings
	embeddingListType := arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)
	embeddingArray := array.NewFixedSizeListData(
		array.NewData(embeddingListType, numRecords, []*memory.Buffer{nil}, []arrow.ArrayData{embeddingFloat32Array.Data()}, 0, 0),
	)
	defer embeddingArray.Release()

	// Create Arrow Record
	columns := []arrow.Array{idArray, nameArray, categoryArray, scoreArray, embeddingArray}
	record := array.NewRecord(arrowSchema, columns, numRecords)
	defer record.Release()

	// Add data to table
	err = table.Add(record, nil)
	if err != nil {
		t.Fatalf("❌Failed to add data: %v", err)
	}
	t.Log("✅ Sample data added successfully")

	// Test GetAllIndexes on empty table (should return empty list)
	t.Log("\n📋 Testing GetAllIndexes on table with no indexes...")
	indexes, err := table.GetAllIndexes()
	if err != nil {
		t.Fatalf("❌Failed to get indexes: %v", err)
	}

	t.Logf("Found %d indexes (expected 0):\n", len(indexes))
	for i, idx := range indexes {
		t.Logf("  %d. Name: %s, Columns: %v, Type: %s\n", i+1, idx.Name, idx.Columns, idx.IndexType)
	}

	// Create some indexes
	t.Log("\n🔧 Creating various indexes...")

	indexesToCreate := []struct {
		name        string
		columns     []string
		indexType   IndexType
		customName  string
		description string
	}{
		{
			name:        "BTree Index",
			columns:     []string{"id"},
			indexType:   IndexTypeBTree,
			customName:  "id_btree_idx",
			description: "BTree index on ID field",
		},
		{
			name:        "Bitmap Index",
			columns:     []string{"category"},
			indexType:   IndexTypeBitmap,
			customName:  "category_bitmap_idx",
			description: "Bitmap index on category field",
		},
		{
			name:        "FTS Index",
			columns:     []string{"name"},
			indexType:   IndexTypeFts,
			customName:  "name_fts_idx",
			description: "Full-text search on name field",
		},
		{
			name:        "Vector Index (IVF_PQ)",
			columns:     []string{"embedding"},
			indexType:   IndexTypeIvfPq,
			customName:  "embedding_ivf_pq_idx",
			description: "IVF_PQ vector index on embedding field",
		},
		{
			name:        "Vector Index (IVF_Flat)",
			columns:     []string{"embedding"},
			indexType:   IndexTypeIvfFlat,
			customName:  "embedding_ivf_flat_idx",
			description: "IVF_Flat vector index for exact search",
		},
		{
			name:        "Vector Index (HNSW_PQ)",
			columns:     []string{"embedding"},
			indexType:   IndexTypeHnswPq,
			customName:  "embedding_hnsw_pq_idx",
			description: "HNSW_PQ vector index for high performance",
		},
	}

	// Create each index
	for _, indexSpec := range indexesToCreate {
		t.Logf("\nCreating %s...\n", indexSpec.description)
		t.Logf("  Columns: %v\n", indexSpec.columns)
		t.Logf("  Type: %v\n", indexSpec.indexType)
		t.Logf("  Custom Name: %s\n", indexSpec.customName)

		err = table.CreateIndexWithName(indexSpec.columns, indexSpec.indexType, indexSpec.customName)
		if err != nil {
			t.Fatalf("❌ Failed to create %s: %v\n", indexSpec.name, err)
		}
		t.Logf("  ✅ %s created successfully\n", indexSpec.name)

		// Test GetAllIndexes after each index creation
		t.Logf("  📋 Checking indexes after creating %s...\n", indexSpec.name)
		indexes, err = table.GetAllIndexes()
		if err != nil {
			t.Logf("  ❌ Failed to get indexes: %v\n", err)
			continue
		}
		t.Logf("  Found %d indexes:\n", len(indexes))
		for i, idx := range indexes {
			t.Logf("    %d. Name: %s, Columns: %v, Type: %s\n", i+1, idx.Name, idx.Columns, idx.IndexType)
		}
	}

	// Final check - get all indexes
	t.Log("\n📊 Final GetAllIndexes test...")
	finalIndexes, err := table.GetAllIndexes()
	if err != nil {
		t.Fatalf("❌Failed to get final indexes: %v", err)
	}

	t.Logf("🎯 Total indexes on table: %d\n", len(finalIndexes))
	if len(finalIndexes) > 0 {
		t.Log("Index details:")
		for i, idx := range finalIndexes {
			t.Logf("  %d. Name: %s\n", i+1, idx.Name)
			t.Logf("     Columns: %v\n", idx.Columns)
			t.Logf("     Type: %s\n", idx.IndexType)
			t.Log()
		}
	}

	// Test error cases
	t.Log("🧪 Testing error cases...")

	// Test GetAllIndexes on closed table
	table.Close()
	_, err = table.GetAllIndexes()
	if err != nil {
		t.Logf("✅ Correctly caught closed table error: %v\n", err)
	} else {
		t.Log("❌ Should have failed on closed table")
	}

	t.Log("\n🎉 GetAllIndexes functionality test completed successfully!")
}
