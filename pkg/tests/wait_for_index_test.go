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

// setupWaitForIndexTable creates a small table with a vector column. Enough
// rows are seeded that CreateIndex(IVF_PQ) has to do a real training pass,
// so WaitForIndex has meaningful work to observe.
func setupWaitForIndexTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_wait_for_index_")
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

	table, err := conn.CreateTable(context.Background(), "wfi", schema)
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

// TestWaitForIndex_ReturnsOnceBuilt — Strategy 4 (Round Trip): CreateIndex
// returns before the index file is fully materialised; WaitForIndex must
// block until num_unindexed_rows == 0, after which IndexStats confirms the
// invariant. A generous timeout keeps the test robust on slow CI workers.
func TestWaitForIndex_ReturnsOnceBuilt(t *testing.T) {
	table, cleanup := setupWaitForIndexTable(t)
	defer cleanup()

	ctx := context.Background()

	require.NoError(t, table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "emb_ivf_pq"))

	require.NoError(t, table.WaitForIndex(ctx, []string{"emb_ivf_pq"}, 60*time.Second))

	stats, err := table.IndexStats(ctx, "emb_ivf_pq")
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Zero(t, stats.NumUnindexedRows, "after WaitForIndex all rows must be indexed")
}

// TestWaitForIndex_MissingIndex_ReturnsError — Strategy 1 (Edge): the
// backend surfaces a typed error when the caller asks for an index that
// doesn't exist; the FFI passes the message through.
func TestWaitForIndex_MissingIndex_ReturnsError(t *testing.T) {
	table, cleanup := setupWaitForIndexTable(t)
	defer cleanup()

	err := table.WaitForIndex(context.Background(), []string{"nonexistent_index"}, 2*time.Second)
	require.Error(t, err, "missing index must surface an error")
}

// TestWaitForIndex_ContextCancelled_ShortCircuits — Strategy 1 (Edge): a
// pre-cancelled ctx must short-circuit before crossing the FFI boundary.
func TestWaitForIndex_ContextCancelled_ShortCircuits(t *testing.T) {
	table, cleanup := setupWaitForIndexTable(t)
	defer cleanup()

	require.NoError(t, table.CreateIndexWithName(context.Background(), []string{"embedding"}, contracts.IndexTypeIvfPq, "emb"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := table.WaitForIndex(ctx, []string{"emb"}, 10*time.Second)
	require.ErrorIs(t, err, context.Canceled)
}

// TestWaitForIndex_ClosedTable_ReturnsError — use-after-close guard.
func TestWaitForIndex_ClosedTable_ReturnsError(t *testing.T) {
	table, cleanup := setupWaitForIndexTable(t)
	table.Close()
	defer cleanup()

	err := table.WaitForIndex(context.Background(), nil, time.Second)
	require.Error(t, err)
}

// TestComputeWaitTimeoutMs covers the deadline-folding logic separately
// so the table-level test doesn't need a LanceDB connection per case.
// Pins the two Codex-flagged invariants:
//   - sub-millisecond positive durations don't truncate to 0 (which the
//     Rust side would interpret as Duration::MAX = wait forever);
//   - ctx deadlines actually narrow the timeout forwarded to FFI, so
//     context.WithTimeout becomes a real upper bound on the wait.
func TestComputeWaitTimeoutMs(t *testing.T) {
	ctxBg := context.Background()
	cases := []struct {
		name    string
		ctx     func() (context.Context, context.CancelFunc)
		timeout time.Duration
		want    uint64
		wantMin uint64
		wantMax uint64
	}{
		{
			name:    "explicit_seconds_no_deadline",
			ctx:     func() (context.Context, context.CancelFunc) { return ctxBg, func() {} },
			timeout: 5 * time.Second,
			want:    5000,
		},
		{
			name:    "zero_no_deadline_unbounded",
			ctx:     func() (context.Context, context.CancelFunc) { return ctxBg, func() {} },
			timeout: 0,
			want:    0,
		},
		{
			name:    "submillisecond_floored_to_1",
			ctx:     func() (context.Context, context.CancelFunc) { return ctxBg, func() {} },
			timeout: 100 * time.Microsecond,
			want:    1,
		},
		{
			name:    "negative_returns_1",
			ctx:     func() (context.Context, context.CancelFunc) { return ctxBg, func() {} },
			timeout: -5 * time.Second,
			want:    1,
		},
		{
			name: "deadline_narrows_zero_timeout",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctxBg, 2*time.Second)
			},
			timeout: 0,
			wantMin: 100, // give ample slack so test isn't flaky
			wantMax: 2000,
		},
		{
			name: "deadline_narrows_larger_timeout",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctxBg, 1*time.Second)
			},
			timeout: 1 * time.Hour,
			wantMin: 100,
			wantMax: 1000,
		},
		{
			name: "expired_deadline_returns_1",
			ctx: func() (context.Context, context.CancelFunc) {
				c, cancel := context.WithDeadline(ctxBg, time.Now().Add(-time.Second))
				return c, cancel
			},
			timeout: 5 * time.Second,
			want:    1,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := c.ctx()
			defer cancel()
			got := internal.ComputeWaitTimeoutMs(ctx, c.timeout)
			if c.wantMax > 0 {
				require.LessOrEqual(t, got, c.wantMax, "got %d ms, want <=%d", got, c.wantMax)
				require.GreaterOrEqual(t, got, c.wantMin, "got %d ms, want >=%d", got, c.wantMin)
			} else {
				require.Equal(t, c.want, got)
			}
		})
	}
}
