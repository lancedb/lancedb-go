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
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// setupPrewarmIndexTable mirrors setupWaitForIndexTable but is duplicated
// here to keep this test file self-contained and avoid coupling the two
// suites — the helper in wait_for_index_test.go is unexported.
func setupPrewarmIndexTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_prewarm_index_")
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

	table, err := conn.CreateTable(context.Background(), "prewarm", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 300
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

// TestPrewarmIndex_AfterBuild_Succeeds — Strategy 4 (Round Trip): build an
// index, wait for it to materialise, then ask the backend to load its
// pages into the cache. The call must accept and return without error;
// pages-loaded count is not exposed by the FFI so we only assert the
// happy path completes. Goes through the optional capability interface
// so the source-compat contract (caller code does `t.(ITablePrewarmIndex)`,
// not `t.PrewarmIndex(...)` directly) is exercised here too.
func TestPrewarmIndex_AfterBuild_Succeeds(t *testing.T) {
	table, cleanup := setupPrewarmIndexTable(t)
	defer cleanup()

	ctx := context.Background()

	require.NoError(t, table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "emb_ivf_pq"))
	require.NoError(t, table.WaitForIndex(ctx, []string{"emb_ivf_pq"}, 60*time.Second))

	var iface contracts.ITable = table
	p, ok := iface.(contracts.ITablePrewarmIndex)
	require.True(t, ok, "*internal.Table must implement ITablePrewarmIndex")
	require.NoError(t, p.PrewarmIndex(ctx, "emb_ivf_pq"))
}

// TestPrewarmIndex_MissingIndex_ReturnsError — Strategy 1 (Edge): asking
// the backend to prewarm an index that was never created surfaces a
// backend error verbatim; the FFI must not silently succeed.
func TestPrewarmIndex_MissingIndex_ReturnsError(t *testing.T) {
	table, cleanup := setupPrewarmIndexTable(t)
	defer cleanup()

	err := table.PrewarmIndex(context.Background(), "nonexistent_index")
	require.Error(t, err, "missing index must surface an error")
}

// TestPrewarmIndex_EmptyName_ReturnsError — Strategy 1 (Edge): the Go
// layer rejects an empty name before crossing the FFI boundary, matching
// the DropIndex guard.
func TestPrewarmIndex_EmptyName_ReturnsError(t *testing.T) {
	table, cleanup := setupPrewarmIndexTable(t)
	defer cleanup()

	err := table.PrewarmIndex(context.Background(), "")
	require.Error(t, err, "empty index name must be rejected before FFI call")
}

// TestPrewarmIndex_ClosedTable_ReturnsError — use-after-close guard,
// matching the rest of the index ops.
func TestPrewarmIndex_ClosedTable_ReturnsError(t *testing.T) {
	table, cleanup := setupPrewarmIndexTable(t)
	table.Close()
	defer cleanup()

	err := table.PrewarmIndex(context.Background(), "emb_ivf_pq")
	require.Error(t, err)
}
