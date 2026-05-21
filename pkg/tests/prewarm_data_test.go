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
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// setupPrewarmDataTable mirrors setupPrewarmIndexTable but stays
// self-contained to keep this test file independent.
func setupPrewarmDataTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_prewarm_data_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(64, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), "prewarm_data", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 64
	pool := memory.NewGoAllocator()

	idB := array.NewInt32Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, 64, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		embB.Append(true)
		for j := 0; j < 64; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()

	if err := table.Add(context.Background(), rec, nil); err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("add: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// expectsLocalNotSupportedError checks that calling prewarm_data on a
// local Native table surfaces a not-supported / remote-only backend
// error. lance has returned two different wordings here across recent
// versions ("prewarm_data is currently only supported on remote tables"
// and a generic "not supported"), so we accept either to stay tolerant
// of upstream message tweaks while still pinning the behavioural
// contract (= local must error out).
func expectsLocalNotSupportedError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err, "local prewarm_data must return an error")
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "remote") && !strings.Contains(lower, "not supported") {
		t.Fatalf("unexpected error message (want substring \"remote\" or \"not supported\"): %v", err)
	}
	// Echo the actual message so CI logs make the upstream wording
	// visible without having to download artifacts. Useful when lance
	// changes the string and the assertion above starts diverging.
	t.Logf("backend error (informational): %v", err)
}

// TestPrewarmData_Local_AllColumns_RemoteOnly — Strategy 3 (Cross
// Validation against upstream policy): local Native tables intentionally
// do not implement prewarm_data; lance v6 surfaces
// "prewarm_data is currently only supported on remote tables." The FFI
// must forward that error verbatim instead of silently succeeding or
// panicking. A nil columns slice exercises the Option::None path
// (= all columns).
func TestPrewarmData_Local_AllColumns_RemoteOnly(t *testing.T) {
	table, cleanup := setupPrewarmDataTable(t)
	defer cleanup()

	var iface contracts.ITable = table
	p, ok := iface.(contracts.ITablePrewarmData)
	require.True(t, ok, "*internal.Table must implement ITablePrewarmData")

	expectsLocalNotSupportedError(t, p.PrewarmData(context.Background(), nil))
}

// TestPrewarmData_Local_NamedColumns_RemoteOnly — Strategy 3: same
// remote-only policy applies when specific columns are listed. Also
// exercises the *const *const c_char → Option::Some(Vec<String>) FFI
// conversion path which is otherwise untested by the "all columns"
// case.
func TestPrewarmData_Local_NamedColumns_RemoteOnly(t *testing.T) {
	table, cleanup := setupPrewarmDataTable(t)
	defer cleanup()

	err := table.PrewarmData(context.Background(), []string{"embedding"})
	expectsLocalNotSupportedError(t, err)
}

// TestPrewarmData_EmptyColumnName_RejectedBeforeFFI — Strategy 1 (Edge):
// the Go layer guards against empty column names before crossing the
// FFI boundary, matching DropIndex / PrewarmIndex behaviour. An empty
// name at any index must surface as a Go error without ever calling
// the Rust side.
//
// The empty name is placed AFTER a valid name so the early-return
// branch also exercises the cstring-leak guard: the defer that frees
// the cArr buffers is registered before the allocation loop, so the
// already-allocated C.CString for "embedding" must still be freed
// when the second iteration short-circuits. The loop is repeated so
// a double-free regression would crash on a subsequent call.
func TestPrewarmData_EmptyColumnName_RejectedBeforeFFI(t *testing.T) {
	table, cleanup := setupPrewarmDataTable(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		err := table.PrewarmData(context.Background(), []string{"embedding", ""})
		require.Error(t, err, "empty column name must be rejected before FFI call (iter %d)", i)
		require.Contains(t, err.Error(), "empty",
			"error should mention the empty name (iter %d); got: %v", i, err)
	}
}

// TestPrewarmData_ClosedTable_ReturnsError — use-after-close guard,
// matching PrewarmIndex.
func TestPrewarmData_ClosedTable_ReturnsError(t *testing.T) {
	table, cleanup := setupPrewarmDataTable(t)
	table.Close()
	defer cleanup()

	err := table.PrewarmData(context.Background(), nil)
	require.Error(t, err, "closed table must surface an error")
	require.Contains(t, err.Error(), "closed",
		"error should mention the table is closed; got: %v", err)
}
