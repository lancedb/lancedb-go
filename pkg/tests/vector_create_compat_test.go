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
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func TestVectorQueryUsesSpecifiedColumn(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_vector_column_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx := context.Background()
	conn, err := lancedb.Connect(ctx, tempDir, nil)
	require.NoError(t, err)
	defer conn.Close()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "left_vec", Type: arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32)},
		{Name: "right_vec", Type: arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32)},
	}, nil)
	publicSchema, err := internal.NewSchema(schema)
	require.NoError(t, err)
	table, err := conn.CreateTable(ctx, "vectors", publicSchema)
	require.NoError(t, err)
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	builder.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	left := builder.Field(1).(*array.FixedSizeListBuilder)
	right := builder.Field(2).(*array.FixedSizeListBuilder)
	left.Append(true)
	left.ValueBuilder().(*array.Float32Builder).AppendValues([]float32{0, 0}, nil)
	right.Append(true)
	right.ValueBuilder().(*array.Float32Builder).AppendValues([]float32{10, 10}, nil)
	left.Append(true)
	left.ValueBuilder().(*array.Float32Builder).AppendValues([]float32{10, 10}, nil)
	right.Append(true)
	right.ValueBuilder().(*array.Float32Builder).AppendValues([]float32{0, 0}, nil)
	record := builder.NewRecord()
	builder.Release()
	require.NoError(t, table.Add(ctx, record, nil))
	record.Release()

	queryVector := []float32{0, 0}
	leftRecord, err := table.(contracts.ITableVectorQuery).VectorQuery("left_vec", queryVector).Limit(1).Execute(ctx)
	require.NoError(t, err, "two vector columns must not require an implicit choice")
	require.Equal(t, int64(1), leftRecord.Column(0).(*array.Int64).Value(0))
	leftRecord.Release()

	rightRecord, err := table.(contracts.ITableVectorQuery).VectorQuery("right_vec", queryVector).Limit(1).Execute(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), rightRecord.Column(0).(*array.Int64).Value(0))
	rightRecord.Release()

	legacyRows, err := table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "left_vec", Vector: queryVector, K: 1},
	})
	require.NoError(t, err)
	require.Len(t, legacyRows, 1)
	require.Equal(t, float64(1), legacyRows[0]["id"])

	_, err = table.(contracts.ITableVectorQuery).VectorQuery("", nil).Limit(1).Execute(ctx)
	require.Error(t, err, "explicit empty VectorQuery must not fall back to a scan")
	_, err = table.(contracts.ITableVectorQuery).VectorQuery("left_vec", nil).Limit(1).Execute(ctx)
	require.Error(t, err)
}

func TestCreateTableRejectsInvalidUTF8Name(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	schema, err := internal.NewSchemaBuilder().AddInt64Field("id", false).Build()
	require.NoError(t, err)
	_, err = conn.CreateTable(ctx, string([]byte{0xff}), schema)
	require.Error(t, err)
	table, err := conn.CreateTable(ctx, "valid_after_invalid_name", schema)
	require.NoError(t, err)
	require.NoError(t, table.Close())
}
