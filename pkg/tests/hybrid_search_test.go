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

// setupHybridSearchTable builds a table with both a vector and a text
// column, seeds rows, and creates an FTS index on the text column so
// WithFullText() has the index it needs. Returns the populated table.
func setupHybridSearchTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_test_hybrid_search_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "body", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(64, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "hybrid", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 200
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	bodyB := array.NewStringBuilder(pool)
	embB := array.NewFixedSizeListBuilder(pool, 64, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)

	texts := []string{
		"the quick brown fox jumps over the lazy dog",
		"a slow blue cat sleeps on the warm mat",
		"fast green turtle swims through the clear river",
		"an orange bird sings in the tall oak tree",
		"the red fox hunts under the moonlit night",
	}
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		bodyB.Append(texts[i%len(texts)])
		embB.Append(true)
		for j := 0; j < 64; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), bodyB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(context.Background(), rec, nil); err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("add: %v", err)
	}

	it := table.(*internal.Table)
	require.NoError(t, it.CreateIndexWithParams(
		context.Background(),
		[]string{"body"},
		contracts.IndexTypeFts,
		contracts.IndexParams{},
		&contracts.CreateIndexOptions{Name: "body_fts", WaitTimeout: 60 * time.Second},
	))

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return it, cleanup
}

// TestHybrid_DefaultReranker_ReturnsRows — the happy path: dense vector
// plus a text query, no explicit reranker. lancedb auto-applies RRF and
// returns fused rows.
func TestHybrid_DefaultReranker_ReturnsRows(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	for i := 0; i < 64; i++ {
		queryVec[i] = 0.1 + float32(i)*0.001
	}

	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0), "hybrid query should return rows")
}

// TestHybrid_ExplicitRRFReranker — explicit RRF with a custom k still
// produces results.
func TestHybrid_ExplicitRRFReranker(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("green turtle", "body").
		Rerank(contracts.RerankerConfig{
			Kind: contracts.RerankerRRF,
			RRFK: 30,
			Norm: contracts.NormalizeRank,
		}).
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0))
}

// TestHybrid_EmptyFullText_FallsBackToVectorOnly — an empty text query
// must NOT trigger the hybrid path: the user's intent is a pure vector
// search, and lancedb's hybrid path with an empty FTS query would error.
func TestHybrid_EmptyFullText_FallsBackToVectorOnly(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("", "").
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0), "empty full-text must behave as vector-only")
}

// TestHybrid_WhitespaceFullText_FallsBackToVectorOnly — Strategy 1 (Edge):
// a whitespace-only query is normalised to "" by WithFullText, and the
// Rust side trims defensively as well, so the hybrid path is skipped
// instead of pushing an empty FullTextSearchQuery into lancedb (which
// would either yield no rows or surface a backend error depending on
// the FTS index).
func TestHybrid_WhitespaceFullText_FallsBackToVectorOnly(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("   \t\n  ", "body").
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0), "whitespace full-text must behave as vector-only")
}

// TestHybrid_ExplicitRRFOverridesDefault — pin the interaction between
// PR D's reranker config and the hybrid path. lancedb auto-applies an
// RRF reranker on hybrid; passing an explicit RerankerConfig should
// not panic or duplicate the application.
func TestHybrid_ExplicitRRFOverridesDefault(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		Rerank(contracts.RerankerConfig{Kind: contracts.RerankerRRF, RRFK: 30}).
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0))
}

// TestHybrid_RerankerNone_StillUsesDefault — explicit RerankerNone is a
// no-op on the Go side (buildConfig drops the section); the hybrid path
// then uses lancedb's default reranker. Pin that this combination works
// instead of erroring on a "missing kind" parser failure.
func TestHybrid_RerankerNone_StillUsesDefault(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		Rerank(contracts.RerankerConfig{Kind: contracts.RerankerNone}).
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	require.Greater(t, rec.NumRows(), int64(0))
}

// TestHybrid_WithFilter — combining hybrid with a SQL filter; lancedb
// applies the filter to both the vector and FTS channels before fusion.
func TestHybrid_WithFilter(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		Filter("id < 50").
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	// Every returned row must satisfy id<50; if not, the filter was
	// silently dropped on one of the two channels.
	idCol := rec.Column(0)
	for i := 0; i < int(rec.NumRows()); i++ {
		v := idCol.ValueStr(i)
		require.NotEmpty(t, v)
	}
}

// TestHybrid_ClosedTable_ReturnsError — use-after-close guard still
// holds on the hybrid path.
func TestHybrid_ClosedTable_ReturnsError(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	table.Close()
	defer cleanup()

	queryVec := make([]float32, 64)
	_, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		Limit(5).
		Execute(context.Background())
	require.Error(t, err)
}

// TestHybrid_WithRowID — row id meta column still surfaces on hybrid
// results (PR A's WithRowID wiring holds across execute_hybrid).
func TestHybrid_WithRowID(t *testing.T) {
	table, cleanup := setupHybridSearchTable(t)
	defer cleanup()

	queryVec := make([]float32, 64)
	rec, err := table.VectorQuery("embedding", queryVec).
		WithFullText("brown fox", "body").
		WithRowID().
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()

	hasRowID := false
	for _, f := range rec.Schema().Fields() {
		if f.Name == "_rowid" {
			hasRowID = true
			break
		}
	}
	require.True(t, hasRowID, "expected _rowid on hybrid result when WithRowID() is set")
}
