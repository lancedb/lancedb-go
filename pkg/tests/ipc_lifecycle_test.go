// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func newIPCResultTable(t *testing.T) (contracts.IConnection, contracts.ITable, *arrow.Schema, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_ipc_lifecycle_")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}
	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "label", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "ipc_items", schema)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	cleanup := func() {
		_ = table.Close()
		_ = conn.Close()
		_ = os.RemoveAll(tempDir)
	}
	return conn, table, arrowSchema, cleanup
}

func appendBatch(t *testing.T, table contracts.ITable, schema *arrow.Schema, ids []int64, labels []string) arrow.Record {
	t.Helper()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	builder.Field(0).(*array.Int64Builder).AppendValues(ids, nil)
	for _, label := range labels {
		builder.Field(1).(*array.StringBuilder).Append(label)
	}
	record := builder.NewRecord()
	builder.Release()
	return record
}

func TestMultiBatchIPCConcatenation(t *testing.T) {
	_, table, schema, cleanup := newIPCResultTable(t)
	defer cleanup()
	ctx := context.Background()

	first := appendBatch(t, table, schema, []int64{1, 2}, []string{"a", "b"})
	second := appendBatch(t, table, schema, []int64{3, 4, 5}, []string{"c", "d", "e"})
	defer first.Release()
	defer second.Release()
	if err := table.AddRecords(ctx, []arrow.Record{first, second}, nil); err != nil {
		t.Fatalf("AddRecords: %v", err)
	}
	if count, err := table.Count(ctx); err != nil || count != 5 {
		t.Fatalf("Count = %d, %v; want 5", count, err)
	}

	qv2 := table.(contracts.ITableQueryV2)
	record, err := qv2.QueryV2().Columns([]string{"id", "label"}).Execute(ctx)
	if err != nil {
		t.Fatalf("QueryV2: %v", err)
	}
	defer record.Release()
	if record.NumRows() != 5 {
		t.Fatalf("single-record rows = %d, want 5", record.NumRows())
	}
	ids := record.Column(0).(*array.Int64)
	for i, want := range []int64{1, 2, 3, 4, 5} {
		if got := ids.Value(i); got != want {
			t.Errorf("id[%d] = %d, want %d", i, got, want)
		}
	}

	legacy, err := table.(*internal.Table).Query().Execute()
	if err != nil {
		t.Fatalf("legacy Execute: %v", err)
	}
	var legacyRows int64
	for _, batch := range legacy {
		legacyRows += batch.NumRows()
		batch.Release()
	}
	if legacyRows != 5 {
		t.Fatalf("legacy batches hold %d rows, want 5", legacyRows)
	}
}

func TestEmptyResultIPCAndSchema(t *testing.T) {
	_, table, schema, cleanup := newIPCResultTable(t)
	defer cleanup()
	ctx := context.Background()

	if count, err := table.Count(ctx); err != nil || count != 0 {
		t.Fatalf("empty Count = %d, %v", count, err)
	}
	rows, err := table.Select(ctx, contracts.QueryConfig{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty Select returned %d rows", len(rows))
	}
	ipcBytes, err := table.(*internal.Table).SelectIPC(ctx, contracts.QueryConfig{})
	if err != nil {
		t.Fatalf("SelectIPC: %v", err)
	}
	if len(ipcBytes) == 0 {
		t.Fatal("SelectIPC returned no schema-only IPC file")
	}
	reader, err := ipc.NewFileReader(bytes.NewReader(ipcBytes))
	if err != nil {
		t.Fatalf("parse schema-only IPC: %v", err)
	}
	defer reader.Close()
	if reader.Schema() == nil || reader.Schema().NumFields() != schema.NumFields() {
		t.Fatalf("schema-only IPC schema = %v", reader.Schema())
	}
	if reader.NumRecords() != 0 {
		t.Fatalf("schema-only IPC has %d records", reader.NumRecords())
	}

	record, err := table.(contracts.ITableQueryV2).QueryV2().Execute(ctx)
	if err != nil {
		t.Fatalf("QueryV2 empty: %v", err)
	}
	if record == nil || record.NumRows() != 0 {
		t.Fatalf("empty single record = %v", record)
	}
	record.Release()
}

func TestRecordsSurviveTableClose(t *testing.T) {
	_, table, schema, cleanup := newIPCResultTable(t)
	defer cleanup()
	ctx := context.Background()

	record := appendBatch(t, table, schema, []int64{7}, []string{"kept"})
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		t.Fatalf("Add: %v", err)
	}
	result, err := table.(contracts.ITableQueryV2).QueryV2().Execute(ctx)
	if err != nil {
		t.Fatalf("QueryV2: %v", err)
	}
	if err := table.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if result.NumRows() != 1 || result.Column(0).(*array.Int64).Value(0) != 7 {
		t.Fatalf("result did not survive table close: %v", result)
	}
	result.Release()

	if err := table.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := table.Select(ctx, contracts.QueryConfig{}); err == nil {
		t.Fatal("Select on closed table succeeded")
	}
	if err := table.Add(ctx, record, nil); err == nil {
		t.Fatal("Add on closed table succeeded")
	}
}

func TestLegacyOverwriteFlagAppends(t *testing.T) {
	_, table, schema, cleanup := newIPCResultTable(t)
	defer cleanup()
	ctx := context.Background()

	first := appendBatch(t, table, schema, []int64{1}, []string{"first"})
	second := appendBatch(t, table, schema, []int64{2}, []string{"second"})
	defer first.Release()
	defer second.Release()
	if err := table.Add(ctx, first, &contracts.AddDataOptions{Mode: contracts.WriteModeOverwrite}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := table.Add(ctx, second, &contracts.AddDataOptions{Mode: contracts.WriteModeOverwrite}); err != nil {
		t.Fatalf("second Add: %v", err)
	}
	if count, err := table.Count(ctx); err != nil || count != 2 {
		t.Fatalf("overwrite flag changed Add semantics: count = %d, %v", count, err)
	}
}

func TestInvalidSQLAndRecovery(t *testing.T) {
	_, table, schema, cleanup := newIPCResultTable(t)
	defer cleanup()
	ctx := context.Background()

	record := appendBatch(t, table, schema, []int64{1}, []string{"x"})
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := table.SelectWithFilter(ctx, "this is not sql"); err == nil {
		t.Fatal("invalid SQL succeeded")
	}
	rows, err := table.SelectWithFilter(ctx, "id = 1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("valid query after failure = %d rows, %v", len(rows), err)
	}
	if err := table.Update(ctx, "id = 1", map[string]interface{}{"label": []string{"composite"}}); err == nil {
		t.Fatal("composite update value succeeded")
	}
}
