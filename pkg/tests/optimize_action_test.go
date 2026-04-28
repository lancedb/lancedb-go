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

// setupOptimizeTable creates a table with a few small fragments so
// Compact / Prune sub-actions have something to chew on.
func setupOptimizeTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_test_optimize_action_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "opt", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create: %v", err)
	}

	pool := memory.NewGoAllocator()
	// Three small Add calls so multiple fragments exist.
	for batch := 0; batch < 3; batch++ {
		idB := array.NewInt32Builder(pool)
		nB := array.NewStringBuilder(pool)
		for i := 0; i < 10; i++ {
			idB.Append(int32(batch*10 + i))
			nB.Append("row")
		}
		rec := array.NewRecord(arrowSchema,
			[]arrow.Array{idB.NewArray(), nB.NewArray()}, 10)
		if err := table.Add(context.Background(), rec, nil); err != nil {
			rec.Release()
			table.Close()
			conn.Close()
			os.RemoveAll(tempDir)
			t.Fatalf("add: %v", err)
		}
		rec.Release()
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// TestOptimize_AllShortcut — the legacy Optimize entry point still
// works (and is internally identical to OptimizeWithAction(All)).
func TestOptimize_AllShortcut(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	stats, err := table.Optimize(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stats)
}

// TestOptimizeWithAction_Compact — Compact runs and returns compaction
// stats, including a non-nil FragmentsRemoved (we appended in 3 batches
// so there's something to merge).
func TestOptimizeWithAction_Compact(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	stats, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizeCompact,
		Compaction: contracts.CompactionParams{
			TargetRowsPerFragment: u64Ptr(100),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.NotNil(t, stats.Compaction, "compact must populate Compaction")
}

// TestOptimizeWithAction_Prune — Prune does not error on a freshly
// created table (no old versions to remove yet, but the call should
// still succeed).
func TestOptimizeWithAction_Prune(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	deleteUnverified := true
	stats, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizePrune,
		Prune: contracts.PruneParams{
			OlderThan:        time.Hour,
			DeleteUnverified: &deleteUnverified,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
}

// TestOptimizeWithAction_Prune_SubSecondOlderThan — pin the sub-second
// floor: 500ms used to truncate to 0 and reach the FFI as
// older_than_seconds=0 (TimeDelta::seconds(0) = immediate cutoff),
// silently pruning very recent versions. The fix floors any positive
// sub-second duration to 1s.
func TestOptimizeWithAction_Prune_SubSecondOlderThan(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	stats, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizePrune,
		Prune: contracts.PruneParams{
			OlderThan: 500 * time.Millisecond,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
}

// TestOptimizeWithAction_Index — Index runs even when no index exists
// on the table; lancedb should treat this as a no-op success.
func TestOptimizeWithAction_Index(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	stats, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizeIndex,
	})
	require.NoError(t, err)
	require.NotNil(t, stats)
}

// TestOptimizeWithAction_UnknownKind_ReturnsError — guard against
// out-of-range casts (e.g. contracts.OptimizeActionKind(99)) reaching
// the FFI; the Go side must surface a normal error.
func TestOptimizeWithAction_UnknownKind_ReturnsError(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	defer cleanup()

	_, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizeActionKind(99),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "OptimizeActionKind")
}

// TestOptimizeWithAction_ClosedTable — use-after-close guard.
func TestOptimizeWithAction_ClosedTable(t *testing.T) {
	table, cleanup := setupOptimizeTable(t)
	table.Close()
	defer cleanup()

	_, err := table.OptimizeWithAction(context.Background(), contracts.OptimizeAction{
		Kind: contracts.OptimizeAll,
	})
	require.Error(t, err)
}

func u64Ptr(v uint64) *uint64 { return &v }
