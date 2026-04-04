// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func setupFTSTestTable(t *testing.T) contracts.ITable {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_fts_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	require.NoError(t, err)

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "title", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "body", Type: arrow.BinaryTypes.String, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	require.NoError(t, err)

	table, err := conn.CreateTable(context.Background(), "test_fts", schema)
	require.NoError(t, err)

	pool := memory.NewGoAllocator()

	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3, 4}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	titleBuilder := array.NewStringBuilder(pool)
	titleBuilder.AppendValues([]string{
		"Introduction to Go",
		"Rust Programming",
		"Go Concurrency Patterns",
		"Python Data Science",
	}, nil)
	titleArray := titleBuilder.NewArray()
	defer titleArray.Release()

	bodyBuilder := array.NewStringBuilder(pool)
	bodyBuilder.AppendValues([]string{
		"Go is a statically typed compiled language",
		"Rust provides memory safety without garbage collection",
		"Goroutines and channels are Go concurrency primitives",
		"Python is widely used for data analysis and machine learning",
	}, nil)
	bodyArray := bodyBuilder.NewArray()
	defer bodyArray.Release()

	record := array.NewRecord(arrowSchema, []arrow.Array{idArray, titleArray, bodyArray}, 4)
	defer record.Release()

	err = table.Add(context.Background(), record, nil)
	require.NoError(t, err)

	// Register cleanup before CreateIndex so resources are freed even if index creation fails
	t.Cleanup(func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	})

	// Create FTS index on title column
	err = table.CreateIndex(context.Background(), []string{"title"}, contracts.IndexTypeFts)
	require.NoError(t, err)

	return table
}

func TestFullTextSearch(t *testing.T) {
	table := setupFTSTestTable(t)

	t.Run("FTS returns matching rows", func(t *testing.T) {
		results, err := table.FullTextSearch(context.Background(), "title", "Go")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2, "Should find at least 2 Go-related titles")
	})

	t.Run("FTS with filter", func(t *testing.T) {
		results, err := table.FullTextSearchWithFilter(context.Background(), "title", "Go", "id < 3")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)
		for _, row := range results {
			id, ok := row["id"].(float64)
			require.True(t, ok, "id field should be float64, got %T", row["id"])
			assert.Less(t, id, 3.0)
		}
	})

	t.Run("FTS with no matches returns empty", func(t *testing.T) {
		results, err := table.FullTextSearch(context.Background(), "title", "JavaScript")
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
