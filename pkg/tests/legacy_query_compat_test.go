// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func TestQueryV2DistinctResultBatches(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_distinct_batches_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx := context.Background()
	conn, err := lancedb.Connect(ctx, tempDir, nil)
	require.NoError(t, err)
	defer conn.Close()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "text", Type: arrow.BinaryTypes.String},
	}, nil)
	publicSchema, err := internal.NewSchema(schema)
	require.NoError(t, err)
	table, err := conn.CreateTable(ctx, "batched", publicSchema)
	require.NoError(t, err)
	defer table.Close()

	for batch := 0; batch < 3; batch++ {
		builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
		ids := builder.Field(0).(*array.Int64Builder)
		texts := builder.Field(1).(*array.StringBuilder)
		for row := 0; row < 3; row++ {
			ids.Append(int64(batch*3 + row + 1))
			texts.Append("row")
		}
		record := builder.NewRecord()
		builder.Release()
		require.NoError(t, table.Add(ctx, record, nil))
		record.Release()
	}

	legacy, err := table.(*internal.Table).Query().Execute()
	require.NoError(t, err)
	require.Len(t, legacy, 3, "legacy Execute must preserve three distinct result batches")
	legacyIDs := make(map[int64]bool)
	for _, batch := range legacy {
		require.Equal(t, int64(3), batch.NumRows())
		column := batch.Column(0).(*array.Int64)
		for i := 0; i < column.Len(); i++ {
			legacyIDs[column.Value(i)] = true
		}
		batch.Release()
	}
	require.Len(t, legacyIDs, 9)

	record, err := table.(contracts.ITableQueryV2).QueryV2().Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, record)
	defer record.Release()
	require.Equal(t, int64(9), record.NumRows(), "QueryV2 must concatenate every result batch")
	ids := record.Column(0).(*array.Int64)
	seen := make(map[int64]bool)
	for i := 0; i < ids.Len(); i++ {
		seen[ids.Value(i)] = true
	}
	for id := int64(1); id <= 9; id++ {
		require.True(t, seen[id], "QueryV2 dropped id %d", id)
	}
}

func TestLegacySelectScopeAndErrors(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()
	ctx := context.Background()

	zero := 0
	negative := -1
	offset := 1
	queryVector := make([]float32, 128)
	for i := range queryVector {
		queryVector[i] = 0.1 + float32(i)*0.001
	}

	rows, err := table.Select(ctx, contracts.QueryConfig{Columns: []string{"id"}, Limit: &zero})
	require.NoError(t, err)
	require.Empty(t, rows, "explicit zero limit must stay empty")

	rows, err = table.Select(ctx, contracts.QueryConfig{Columns: []string{"id"}, Limit: &negative, Offset: &negative})
	require.NoError(t, err)
	require.Len(t, rows, 5, "negative legacy limit/offset were ignored")

	negativeK := -1
	rows, err = table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "embedding", Vector: queryVector, K: negativeK},
	})
	require.NoError(t, err)
	require.Len(t, rows, 5, "negative legacy K fell through to the ordinary query path")

	zeroK := 0
	_, err = table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "embedding", Vector: queryVector, K: zeroK},
	})
	require.Error(t, err, "legacy zero K remains an SDK error")

	baseRows, err := table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "embedding", Vector: queryVector, K: 2},
	})
	require.NoError(t, err)
	offsetRows, err := table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "embedding", Vector: queryVector, K: 2},
		Offset:       &offset,
	})
	require.NoError(t, err)
	require.Equal(t, rowIDs(baseRows), rowIDs(offsetRows), "legacy vector Offset must be ignored")

	hybridRows, err := table.Select(ctx, contracts.QueryConfig{
		Columns:      []string{"id"},
		VectorSearch: &contracts.VectorSearch{Column: "embedding", Vector: queryVector, K: 2},
		FTSSearch:    &contracts.FTSSearch{Column: "name", Query: "Alice"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, hybridRows)
	_, hasDistance := hybridRows[0]["_distance"]
	_, hasScore := hybridRows[0]["_score"]
	_, hasRelevance := hybridRows[0]["_relevance_score"]
	require.True(t, hasDistance, "legacy vector-over-FTS result must expose _distance")
	require.False(t, hasScore || hasRelevance, "legacy vector-over-FTS must not become a hybrid search")

	for _, query := range []string{"", "Go"} {
		_, err := table.Select(ctx, contracts.QueryConfig{FTSSearch: &contracts.FTSSearch{Column: "name", Query: query}})
		require.Error(t, err, "legacy Select FTS must keep the v0.1.2 unsupported error")
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	rows, err = table.Select(canceled, contracts.QueryConfig{Columns: []string{"id"}})
	require.NoError(t, err)
	require.Len(t, rows, 5, "legacy Select keeps executing with an already-canceled context")
}

func rowIDs(rows []map[string]interface{}) []float64 {
	out := make([]float64, len(rows))
	for i, row := range rows {
		out[i], _ = row["id"].(float64)
	}
	return out
}

func TestLegacyRowValueAndIndexContracts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_row_contracts_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	ctx := context.Background()
	conn, err := lancedb.Connect(ctx, tempDir, nil)
	require.NoError(t, err)
	defer conn.Close()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "i8", Type: arrow.PrimitiveTypes.Int8},
		{Name: "u8", Type: arrow.PrimitiveTypes.Uint8},
		{Name: "f64", Type: arrow.PrimitiveTypes.Float64},
		{Name: "bin", Type: arrow.BinaryTypes.Binary},
		{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}},
		{Name: "labels", Type: arrow.ListOf(arrow.BinaryTypes.String)},
		{Name: "text", Type: arrow.BinaryTypes.String},
	}, nil)
	publicSchema, err := internal.NewSchema(schema)
	require.NoError(t, err)
	table, err := conn.CreateTable(ctx, "rows", publicSchema)
	require.NoError(t, err)
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	builder.Field(0).(*array.Int64Builder).Append(1)
	builder.Field(1).(*array.Int8Builder).Append(7)
	builder.Field(2).(*array.Uint8Builder).Append(8)
	builder.Field(3).(*array.Float64Builder).Append(math.NaN())
	builder.Field(4).(*array.BinaryBuilder).Append([]byte("bytes"))
	builder.Field(5).(*array.TimestampBuilder).AppendTime(time.Unix(0, 0).UTC())
	list := builder.Field(6).(*array.ListBuilder)
	list.Append(true)
	list.ValueBuilder().(*array.StringBuilder).AppendValues([]string{"a", "b"}, nil)
	builder.Field(7).(*array.StringBuilder).Append("fts text")
	record := builder.NewRecord()
	builder.Release()
	require.NoError(t, table.Add(ctx, record, nil))
	record.Release()

	rows, err := table.Select(ctx, contracts.QueryConfig{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	row := rows[0]
	require.Equal(t, float64(1), row["id"])
	require.Equal(t, "Unsupported type: Int8", row["i8"])
	require.Equal(t, "Unsupported type: UInt8", row["u8"])
	require.Nil(t, row["f64"], "non-finite float must remain nil after the legacy JSON conversion")
	require.Equal(t, "Unsupported type: Binary", row["bin"])
	require.Equal(t, "Unsupported type: Timestamp(Microsecond, None)", row["ts"])
	require.Equal(t, `Unsupported type: List(Field { name: "item", data_type: Utf8, nullable: true, dict_id: 0, dict_is_ordered: false, metadata: {} })`, row["labels"])

	require.NoError(t, table.CreateIndex(ctx, []string{"text"}, contracts.IndexTypeFts))
	indexes, err := table.GetAllIndexes(ctx)
	require.NoError(t, err)
	foundFTS := false
	for _, index := range indexes {
		if index.Name == "text_idx" || index.IndexType == "FTS" {
			if index.IndexType == "FTS" {
				foundFTS = true
			}
		}
	}
	require.True(t, foundFTS, "GetAllIndexes must preserve the v0.1.2 FTS spelling, got %+v", indexes)
}

func TestClosedTableAndConnectionQueryContracts(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, table.Close())
	require.Error(t, table.Add(ctx, nil, nil), "closed table Add(nil) must fail")

	conn, cleanupDB := setupTestDB(t)
	defer cleanupDB()
	schema, err := internal.NewSchemaBuilder().AddInt64Field("id", false).Build()
	require.NoError(t, err)
	live, err := conn.CreateTable(ctx, "live", schema)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	count, countErr := live.Count(ctx)
	require.NoError(t, countErr)
	require.Equal(t, int64(0), count, "Count after connection close keeps the v0.1.2 behavior")

	results, errs := live.Query().ExecuteAsync()
	rows, rowsOpen := <-results
	queryErr, errsOpen := <-errs
	require.False(t, rowsOpen)
	require.Nil(t, rows)
	require.True(t, errsOpen)
	require.Error(t, queryErr)

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	canceledConn, err := lancedb.Connect(canceled, t.TempDir(), nil)
	require.NoError(t, err, "legacy Connect must keep ignoring an already-canceled context")
	require.NotNil(t, canceledConn)
	_ = canceledConn.Close()

	_, err = live.(contracts.ITableQueryV2).SelectV2(canceled, contracts.QueryConfigV2{})
	require.Error(t, err, "additive SelectV2 still honors a canceled context")
	_ = live.Close()
}
