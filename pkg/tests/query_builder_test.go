// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// setupVectorQueryTestTable creates a test table with a vector embedding column for VectorQuery tests.
func setupVectorQueryTestTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_vector_query_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), "test_vq", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create table: %v", err)
	}

	pool := memory.NewGoAllocator()
	numRecords := 5

	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3, 4, 5}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues([]string{"Alice", "Bob", "Charlie", "Diana", "Eve"}, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	scoreBuilder := array.NewFloat64Builder(pool)
	scoreBuilder.AppendValues([]float64{95.5, 87.2, 92.8, 88.9, 94.1}, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	embeddingValues := make([]float32, numRecords*128)
	for i := 0; i < numRecords; i++ {
		for j := 0; j < 128; j++ {
			embeddingValues[i*128+j] = float32(i)*0.1 + float32(j)*0.001
		}
	}
	embeddingFloat32Builder := array.NewFloat32Builder(pool)
	embeddingFloat32Builder.AppendValues(embeddingValues, nil)
	embeddingFloat32Array := embeddingFloat32Builder.NewArray()
	defer embeddingFloat32Array.Release()

	embeddingListType := arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)
	embeddingArray := array.NewFixedSizeListData(
		array.NewData(embeddingListType, numRecords, []*memory.Buffer{nil}, []arrow.ArrayData{embeddingFloat32Array.Data()}, 0, 0),
	)
	defer embeddingArray.Release()

	columns := []arrow.Array{idArray, nameArray, scoreArray, embeddingArray}
	record := array.NewRecord(arrowSchema, columns, int64(numRecords))
	defer record.Release()

	err = table.Add(context.Background(), record, nil)
	if err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to add data: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}

	return table.(*internal.Table), cleanup
}

// setupQueryTestTable creates a test table with sample data for query builder tests.
// Returns the table and a cleanup function.
func setupQueryTestTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_query_builder_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), "test_qb", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create table: %v", err)
	}

	pool := memory.NewGoAllocator()
	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3, 4, 5}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues([]string{"Alice", "Bob", "Charlie", "Diana", "Eve"}, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	scoreBuilder := array.NewFloat64Builder(pool)
	scoreBuilder.AppendValues([]float64{95.5, 87.2, 92.8, 88.9, 94.1}, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	columns := []arrow.Array{idArray, nameArray, scoreArray}
	record := array.NewRecord(arrowSchema, columns, 5)
	defer record.Release()

	err = table.Add(context.Background(), record, nil)
	if err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to add data: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}

	return table.(*internal.Table), cleanup
}

func TestQueryBuilderExecute(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	t.Run("Execute with no options returns all rows", func(t *testing.T) {
		record, err := table.Query().Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(5), record.NumRows())
	})

	t.Run("Execute with filter", func(t *testing.T) {
		record, err := table.Query().Filter("score > 90").Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
		scoreIdx := record.Schema().FieldIndices("score")
		require.Len(t, scoreIdx, 1)
		scoreCol := record.Column(scoreIdx[0]).(*array.Float64)
		for i := 0; i < int(record.NumRows()); i++ {
			assert.Greater(t, scoreCol.Value(i), 90.0)
		}
	})

	t.Run("Execute with multiple filters joined by AND", func(t *testing.T) {
		record, err := table.Query().
			Filter("score > 85").
			Filter("score < 95").
			Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(4), record.NumRows())
		scoreIdx := record.Schema().FieldIndices("score")
		require.Len(t, scoreIdx, 1)
		scoreCol := record.Column(scoreIdx[0]).(*array.Float64)
		for i := 0; i < int(record.NumRows()); i++ {
			assert.Greater(t, scoreCol.Value(i), 85.0)
			assert.Less(t, scoreCol.Value(i), 95.0)
		}
	})

	t.Run("Execute with limit", func(t *testing.T) {
		record, err := table.Query().Limit(2).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(2), record.NumRows())
	})

	t.Run("Execute with offset", func(t *testing.T) {
		record, err := table.Query().Offset(2).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("Execute with columns", func(t *testing.T) {
		record, err := table.Query().Columns([]string{"id", "name"}).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(5), record.NumRows())
		schema := record.Schema()
		assert.Equal(t, 2, len(schema.Fields()))
		assert.True(t, schema.HasField("id"))
		assert.True(t, schema.HasField("name"))
	})

	t.Run("Execute with filter and limit chained", func(t *testing.T) {
		record, err := table.Query().Filter("score > 85").Limit(2).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(2), record.NumRows())
	})

	t.Run("Execute on closed table returns error", func(t *testing.T) {
		closedTable, closedCleanup := setupQueryTestTable(t)
		closedTable.Close()
		defer closedCleanup()

		_, err := closedTable.Query().Execute(context.Background())
		require.Error(t, err)
	})
}

func TestQueryBuilderExecuteAsync(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	t.Run("ExecuteAsync returns results on channel", func(t *testing.T) {
		resultChan, errChan := table.Query().Filter("score > 90").ExecuteAsync(context.Background())

		select {
		case record, ok := <-resultChan:
			if !ok {
				// closed-empty: select raced; drain errChan for actual error
				err := <-errChan
				t.Fatalf("ExecuteAsync failed: %v", err)
			}
			defer record.Release()
			assert.Equal(t, int64(3), record.NumRows())
		case err, ok := <-errChan:
			if ok {
				t.Fatalf("ExecuteAsync failed: %v", err)
			}
			// closed-empty: select raced; results must be on resultChan
			record := <-resultChan
			defer record.Release()
			assert.Equal(t, int64(3), record.NumRows())
		case <-time.After(5 * time.Second):
			t.Fatal("ExecuteAsync timed out")
		}
	})

	t.Run("ExecuteAsync on closed table returns error", func(t *testing.T) {
		closedTable, closedCleanup := setupQueryTestTable(t)
		closedTable.Close()
		defer closedCleanup()

		resultChan, errChan := closedTable.Query().ExecuteAsync(context.Background())

		select {
		case record, ok := <-resultChan:
			if ok {
				defer record.Release()
				t.Fatalf("Expected error, got record with %d rows", record.NumRows())
			}
			// closed-empty: select raced; drain errChan for actual error
			err := <-errChan
			require.Error(t, err)
		case err, ok := <-errChan:
			if !ok {
				// closed-empty: select raced; results must be on resultChan
				record := <-resultChan
				if record != nil {
					defer record.Release()
				}
				t.Fatalf("Expected error, got record")
			}
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("ExecuteAsync timed out waiting for error")
		}
	})
}

func TestVectorQueryBuilder(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	for j := 0; j < 128; j++ {
		queryVec[j] = float32(j) * 0.001
	}

	t.Run("Limit returns results", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).Limit(3).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("Limit with Filter returns filtered results", func(t *testing.T) {
		// score > 93 matches only Alice(95.5) and Eve(94.1), proving the filter
		// actually reduces results rather than trivially matching all rows.
		record, err := table.VectorQuery("embedding", queryVec).Limit(5).Filter("score > 93").Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(2), record.NumRows())
		scoreIdx := record.Schema().FieldIndices("score")
		require.Len(t, scoreIdx, 1)
		scoreCol := record.Column(scoreIdx[0]).(*array.Float64)
		for i := 0; i < int(record.NumRows()); i++ {
			assert.Greater(t, scoreCol.Value(i), 93.0)
		}
	})

	t.Run("Without Limit returns error", func(t *testing.T) {
		_, err := table.VectorQuery("embedding", queryVec).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vector search requires a positive K value")
	})

	t.Run("Zero Limit returns error", func(t *testing.T) {
		_, err := table.VectorQuery("embedding", queryVec).Limit(0).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "K must be a positive integer")
	})

	t.Run("Negative Limit returns error", func(t *testing.T) {
		_, err := table.VectorQuery("embedding", queryVec).Limit(-5).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "K must be a positive integer")
	})

	t.Run("Columns restricts returned fields", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).Limit(3).Columns([]string{"id", "name"}).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		schema := record.Schema()
		assert.True(t, schema.HasField("id"))
		assert.True(t, schema.HasField("name"))
		assert.False(t, schema.HasField("score"))
		assert.False(t, schema.HasField("embedding"))
	})

	t.Run("Nil vector returns error", func(t *testing.T) {
		_, err := table.VectorQuery("embedding", nil).Limit(3).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty query vector")
	})

	t.Run("Empty vector returns error", func(t *testing.T) {
		_, err := table.VectorQuery("embedding", []float32{}).Limit(3).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty query vector")
	})

	t.Run("Empty column name returns error", func(t *testing.T) {
		_, err := table.VectorQuery("", queryVec).Limit(3).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty column name")
	})

	t.Run("Closed table returns error", func(t *testing.T) {
		closedTable, closedCleanup := setupVectorQueryTestTable(t)
		closedTable.Close()
		defer closedCleanup()

		_, err := closedTable.VectorQuery("embedding", queryVec).Limit(3).Execute(context.Background())
		require.Error(t, err)
	})

	t.Run("ApplyOptions sets limit via MaxResults", func(t *testing.T) {
		opts := &contracts.QueryOptions{MaxResults: 2}
		record, err := table.VectorQuery("embedding", queryVec).ApplyOptions(opts).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(2), record.NumRows())
	})

	t.Run("ApplyOptions without MaxResults requires explicit Limit", func(t *testing.T) {
		opts := &contracts.QueryOptions{}
		_, err := table.VectorQuery("embedding", queryVec).ApplyOptions(opts).Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a positive K value")
	})

	t.Run("ExecuteAsync returns results on channel", func(t *testing.T) {
		resultChan, errChan := table.VectorQuery("embedding", queryVec).Limit(3).ExecuteAsync(context.Background())

		select {
		case record, ok := <-resultChan:
			if !ok {
				err := <-errChan
				t.Fatalf("ExecuteAsync failed: %v", err)
			}
			defer record.Release()
			assert.Equal(t, int64(3), record.NumRows())
		case err, ok := <-errChan:
			if ok {
				t.Fatalf("ExecuteAsync failed: %v", err)
			}
			record := <-resultChan
			defer record.Release()
			assert.Equal(t, int64(3), record.NumRows())
		case <-time.After(5 * time.Second):
			t.Fatal("ExecuteAsync timed out")
		}
	})

	t.Run("DistanceType L2 executes successfully", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).
			Limit(3).
			DistanceType(contracts.DistanceTypeL2).
			Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("DistanceType Cosine executes successfully", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).
			Limit(3).
			DistanceType(contracts.DistanceTypeCosine).
			Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("DistanceType Dot executes successfully", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).
			Limit(3).
			DistanceType(contracts.DistanceTypeDot).
			Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("Default distance type works without explicit set", func(t *testing.T) {
		record, err := table.VectorQuery("embedding", queryVec).Limit(3).Execute(context.Background())
		require.NoError(t, err)
		require.NotNil(t, record)
		defer record.Release()
		assert.Equal(t, int64(3), record.NumRows())
	})

	t.Run("ExecuteAsync on closed table returns error on channel", func(t *testing.T) {
		closedTable, closedCleanup := setupVectorQueryTestTable(t)
		closedTable.Close()
		defer closedCleanup()

		resultChan, errChan := closedTable.VectorQuery("embedding", queryVec).Limit(3).ExecuteAsync(context.Background())

		select {
		case record, ok := <-resultChan:
			if ok {
				defer record.Release()
				t.Fatalf("Expected error, got record with %d rows", record.NumRows())
			}
			err := <-errChan
			require.Error(t, err)
		case err, ok := <-errChan:
			if !ok {
				record := <-resultChan
				if record != nil {
					defer record.Release()
				}
				t.Fatalf("Expected error, got record")
			}
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("ExecuteAsync timed out waiting for error")
		}
	})
}
