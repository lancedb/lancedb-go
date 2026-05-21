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

// setupWriteParallelismTable creates a small int32-only table that the
// write_parallelism tests can reuse: no embedding column needed since the
// parallelism knob applies at the writer level, not the column dtype.
func setupWriteParallelismTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_test_wpar_")
	require.NoError(t, err)
	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "wpar", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create: %v", err)
	}
	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// makeIDBatch builds a record batch of n rows with id = [0..n).
func makeIDBatch(t *testing.T, n int) (arrow.Record, arrow.Schema) {
	t.Helper()
	pool := memory.NewGoAllocator()
	fields := []arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false}}
	arrowSchema := arrow.NewSchema(fields, nil)
	b := array.NewInt32Builder(pool)
	for i := 0; i < n; i++ {
		b.Append(int32(i))
	}
	col := b.NewArray()
	rec := array.NewRecord(arrowSchema, []arrow.Array{col}, int64(n))
	return rec, *arrowSchema
}

// uintPtr returns a pointer to the literal value.
func uintPtr(v uint) *uint { return &v }

// TestAddRecords_NilOptions_UsesLanceDefault — Strategy 4 (Round Trip):
// pass options=nil so the legacy FFI path is exercised; row count should
// match the input, exactly as before this PR.
func TestAddRecords_NilOptions_UsesLanceDefault(t *testing.T) {
	table, cleanup := setupWriteParallelismTable(t)
	defer cleanup()

	rec, _ := makeIDBatch(t, 32)
	defer rec.Release()

	require.NoError(t, table.AddRecords(context.Background(), []arrow.Record{rec}, nil))

	got, err := table.Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(32), got)
}

// TestAddRecords_WithExplicitParallelism — Strategy 3 (Cross Validation):
// pass WriteParallelism=4 so the options-aware FFI path is exercised. The
// final row count must still match the input — the knob is a perf knob,
// not a correctness knob.
func TestAddRecords_WithExplicitParallelism(t *testing.T) {
	table, cleanup := setupWriteParallelismTable(t)
	defer cleanup()

	rec, _ := makeIDBatch(t, 100)
	defer rec.Release()

	opts := &contracts.AddDataOptions{WriteParallelism: uintPtr(4)}
	require.NoError(t, table.AddRecords(context.Background(), []arrow.Record{rec}, opts))

	got, err := table.Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(100), got)
}

// TestAddRecords_SerialFallback — Strategy 1 (Edge): WriteParallelism=1
// explicitly disables parallel writers. The result must be identical to
// the parallel path.
func TestAddRecords_SerialFallback(t *testing.T) {
	table, cleanup := setupWriteParallelismTable(t)
	defer cleanup()

	rec, _ := makeIDBatch(t, 50)
	defer rec.Release()

	opts := &contracts.AddDataOptions{WriteParallelism: uintPtr(1)}
	require.NoError(t, table.AddRecords(context.Background(), []arrow.Record{rec}, opts))

	got, err := table.Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(50), got)
}

// TestAddRecords_ZeroParallelism_Rejected — Strategy 1 (Edge): 0 is
// the only invalid value lance accepts (it short-circuits to a backend
// error). The Go layer catches it before the FFI call.
func TestAddRecords_ZeroParallelism_Rejected(t *testing.T) {
	table, cleanup := setupWriteParallelismTable(t)
	defer cleanup()

	rec, _ := makeIDBatch(t, 4)
	defer rec.Release()

	opts := &contracts.AddDataOptions{WriteParallelism: uintPtr(0)}
	err := table.AddRecords(context.Background(), []arrow.Record{rec}, opts)
	require.Error(t, err, "WriteParallelism=0 must be rejected before the FFI call")
	require.Contains(t, err.Error(), "WriteParallelism",
		"error message should name the field; got: %v", err)
}
