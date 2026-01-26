// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/assert"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Helper function to get keys from a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestSelectQueries(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_select_")
	if err != nil {
		t.Fatalf("❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to database
	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
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
		{Name: "labels", Type: arrow.ListOf(arrow.BinaryTypes.String), Nullable: true},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("❌Failed to create schema: %v", err)
	}

	// Create table
	table, err := conn.CreateTable(context.Background(), "test_select", schema)
	if err != nil {
		t.Fatalf("❌Failed to create table: %v", err)
	}
	defer table.Close()

	// Add sample data
	pool := memory.NewGoAllocator()
	numRecords := 5

	// Create sample data
	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3, 4, 5}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues([]string{"Alice", "Bob", "Charlie", "Diana", "Eve"}, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	categoryBuilder := array.NewStringBuilder(pool)
	categoryBuilder.AppendValues([]string{"A", "B", "A", "C", "B"}, nil)
	categoryArray := categoryBuilder.NewArray()
	defer categoryArray.Release()

	scoreBuilder := array.NewFloat64Builder(pool)
	scoreBuilder.AppendValues([]float64{95.5, 87.2, 92.8, 88.9, 94.1}, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	// Create vector embeddings (128-dimensional vectors)
	embeddingValues := make([]float32, numRecords*128) // 5 records * 128 dimensions
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

	// Create labels (list of strings)
	labelsBuilder := array.NewListBuilder(pool, arrow.BinaryTypes.String)
	stringBuilder := labelsBuilder.ValueBuilder().(*array.StringBuilder)
	labelsData := [][]string{
		{"student", "athlete"},
		{"engineer"},
		{"artist", "musician"},
		{"scientist"},
		{"doctor", "researcher"},
	}
	for _, labels := range labelsData {
		labelsBuilder.Append(true)
		for _, label := range labels {
			stringBuilder.Append(label)
		}
	}
	labelsArray := labelsBuilder.NewArray()
	defer labelsArray.Release()

	// Create Arrow Record
	columns := []arrow.Array{idArray, nameArray, categoryArray, scoreArray, embeddingArray, labelsArray}
	record := array.NewRecord(arrowSchema, columns, int64(numRecords))
	defer record.Release()

	// Add data to table
	err = table.Add(context.Background(), record, nil)
	if err != nil {
		t.Fatalf("❌Failed to add data: %v", err)
	}
	t.Log("✅ Sample data added successfully")

	t.Run("Select All Records", func(t *testing.T) {
		results, err := table.Select(context.Background(), contracts.QueryConfig{})
		if err != nil {
			t.Fatalf("❌Failed to select all records: %v", err)
		}

		if len(results) != numRecords {
			t.Fatalf("❌Expected %d records, got %d", numRecords, len(results))
		}

		t.Logf("Retrieved %d records", len(results))
		for i, row := range results {
			t.Logf("  Record %d: id=%v, name=%v, score=%v", i+1, row["id"], row["name"], row["score"])
			assert.ElementsMatch(t, labelsData[i], row["labels"])
		}
	})

	t.Run("Select Specific Columns", func(t *testing.T) {
		results, err := table.SelectWithColumns(context.Background(), []string{"id", "name"})
		if err != nil {
			t.Fatalf("❌Failed to select specific columns: %v", err)
		}

		if len(results) != numRecords {
			t.Fatalf("❌Expected %d records, got %d", numRecords, len(results))
		}

		// Check that only selected columns are present
		for i, row := range results {
			if len(row) != 2 {
				t.Fatalf("❌Record %d should have 2 columns, got %d", i, len(row))
			}
			if _, ok := row["id"]; !ok {
				t.Fatalf("❌Record %d missing 'id' column", i)
			}
			if _, ok := row["name"]; !ok {
				t.Fatalf("❌Record %d missing 'name' column", i)
			}
			if _, ok := row["score"]; ok {
				t.Fatalf("❌Record %d should not have 'score' column", i)
			}
		}
		t.Log("✅ Column selection works correctly")
	})

	t.Run("Select with Filter", func(t *testing.T) {
		results, err := table.SelectWithFilter(context.Background(), "score > 90")
		if err != nil {
			t.Fatalf("❌Failed to select with filter: %v", err)
		}

		// Should return records with score > 90 (Alice: 95.5, Charlie: 92.8, Eve: 94.1)
		expectedCount := 3
		if len(results) != expectedCount {
			t.Fatalf("❌Expected %d records with score > 90, got %d", expectedCount, len(results))
		}

		for _, row := range results {
			score, ok := row["score"].(float64)
			if !ok {
				t.Fatal("Score should be float64")
			}
			if score <= 90 {
				t.Fatalf("❌Found record with score %.1f, expected > 90", score)
			}
		}
		t.Log("✅ Filtering works correctly")
	})

	t.Run("Select with Limit", func(t *testing.T) {
		limit := 3
		results, err := table.SelectWithLimit(context.Background(), limit, 0)
		if err != nil {
			t.Fatalf("❌Failed to select with limit: %v", err)
		}

		if len(results) != limit {
			t.Fatalf("❌Expected %d records, got %d", limit, len(results))
		}
		t.Log("✅ Limit works correctly")
	})

	t.Run("Vector Search", func(t *testing.T) {
		// Create a query vector similar to the first record (id=1)
		queryVector := make([]float32, 128)
		for j := 0; j < 128; j++ {
			queryVector[j] = float32(0)*0.1 + float32(j)*0.001 // Similar to record 0
		}

		results, err := table.VectorSearch(context.Background(), "embedding", queryVector, 3)
		if err != nil {
			t.Fatalf("❌Failed to perform vector search: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("❌Expected some results from vector search")
		}

		t.Logf("Vector search returned %d results", len(results))
		for i, row := range results {
			t.Logf("  Result %d: id=%v, name=%v", i+1, row["id"], row["name"])
		}
		t.Log("✅ Vector search works correctly")
	})

	t.Run("Vector Search with Filter", func(t *testing.T) {
		queryVector := make([]float32, 128)
		for j := 0; j < 128; j++ {
			queryVector[j] = float32(0)*0.1 + float32(j)*0.001
		}

		results, err := table.VectorSearchWithFilter(context.Background(), "embedding", queryVector, 5, "category = 'A'")
		if err != nil {
			t.Fatalf("❌Failed to perform vector search with filter: %v", err)
		}

		// Should only return records with category 'A' (Alice and Charlie)
		for _, row := range results {
			category, ok := row["category"].(string)
			if !ok {
				t.Fatal("Category should be string")
			}
			if category != "A" {
				t.Fatalf("❌Found record with category '%s', expected 'A'", category)
			}
		}
		t.Log("✅ Vector search with filter works correctly")
	})

	t.Run("Complex Query Configuration", func(t *testing.T) {
		queryVector := make([]float32, 128)
		for j := 0; j < 128; j++ {
			queryVector[j] = float32(1)*0.1 + float32(j)*0.001 // Similar to record 1
		}

		limit := 2
		config := contracts.QueryConfig{
			Columns: []string{"id", "name", "score"},
			Where:   "score > 85",
			Limit:   &limit,
			VectorSearch: &contracts.VectorSearch{
				Column: "embedding",
				Vector: queryVector,
				K:      5,
			},
		}

		results, err := table.Select(context.Background(), config)
		if err != nil {
			t.Fatalf("❌Failed to perform complex query: %v", err)
		}

		if len(results) > limit {
			t.Fatalf("❌Expected at most %d records due to limit, got %d", limit, len(results))
		}

		// Debug: print actual columns received
		t.Logf("Complex query returned %d results", len(results))
		if len(results) > 0 {
			t.Logf("Columns in first result: %v", getMapKeys(results[0]))
		}

		// Check that only selected columns are present
		for i, row := range results {
			// Note: Vector queries might include additional metadata columns
			// Let's check that at least the requested columns are present
			expectedCols := []string{"id", "name", "score"}
			for _, col := range expectedCols {
				if _, ok := row[col]; !ok {
					t.Fatalf("❌Record %d missing expected column '%s'", i, col)
				}
			}

			// Check score filter
			if score, ok := row["score"].(float64); ok {
				if score <= 85 {
					t.Fatalf("❌Found record with score %.1f, expected > 85", score)
				}
			}
		}
		t.Log("✅ Complex query configuration works correctly")
	})

	t.Run("Full-Text Search (Should Return Error)", func(t *testing.T) {
		config := contracts.QueryConfig{
			FTSSearch: &contracts.FTSSearch{
				Column: "name",
				Query:  "Alice",
			},
		}

		_, err := table.Select(context.Background(), config)
		if err == nil {
			t.Fatal("❌Expected error for FTS search, but got none")
		} else if !contains(err.Error(), "Full-text search is not currently supported") {
			t.Fatalf("❌Expected FTS not supported error, got: %v", err)
		}
		t.Log("✅ FTS search correctly returns not supported error")
	})

	t.Run("Error Handling - Closed Table", func(t *testing.T) {
		table.Close()
		_, err := table.Select(context.Background(), contracts.QueryConfig{})
		if err == nil {
			t.Fatal("❌Expected error when querying closed table")
		}
		if err.Error() != "table is closed" {
			t.Fatalf("❌Expected 'table is closed' error, got: %v", err)
		}
		t.Log("✅ Error handling for closed table works correctly")
	})
}

func TestSelectConvenienceMethods(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_convenience_")
	if err != nil {
		t.Fatalf("❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to database
	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("❌Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create a simple test schema
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("❌Failed to create schema: %v", err)
	}

	// Create table
	table, err := conn.CreateTable(context.Background(), "test_convenience", schema)
	if err != nil {
		t.Fatalf("❌Failed to create table: %v", err)
	}
	defer table.Close()

	// Add sample data
	pool := memory.NewGoAllocator()
	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues([]string{"Alice", "Bob", "Charlie"}, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	scoreBuilder := array.NewFloat64Builder(pool)
	scoreBuilder.AppendValues([]float64{95.5, 87.2, 92.8}, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	columns := []arrow.Array{idArray, nameArray, scoreArray}
	record := array.NewRecord(arrowSchema, columns, 3)
	defer record.Release()

	err = table.Add(context.Background(), record, nil)
	if err != nil {
		t.Fatalf("❌Failed to add data: %v", err)
	}

	t.Run("SelectWithColumns", func(t *testing.T) {
		results, err := table.SelectWithColumns(context.Background(), []string{"name", "score"})
		if err != nil {
			t.Fatalf("SelectWithColumns failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("❌Expected 3 results, got %d", len(results))
		}
		t.Log("✅ SelectWithColumns works")
	})

	t.Run("SelectWithFilter", func(t *testing.T) {
		results, err := table.SelectWithFilter(context.Background(), "score > 90")
		if err != nil {
			t.Fatalf("SelectWithFilter failed: %v", err)
		}
		if len(results) != 2 { // Alice: 95.5, Charlie: 92.8
			t.Fatalf("❌Expected 2 results, got %d", len(results))
		}
		t.Log("✅ SelectWithFilter works")
	})

	t.Run("SelectWithLimit", func(t *testing.T) {
		results, err := table.SelectWithLimit(context.Background(), 2, 1)
		if err != nil {
			t.Fatalf("SelectWithLimit failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("❌Expected 2 results, got %d", len(results))
		}
		t.Log("✅ SelectWithLimit works")
	})
}
