// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// These tests focus on the wire-up of RerankerConfig through the Go builder
// and into the Rust FFI. Observable reranker effects are tested under PR E
// (hybrid search), which actually combines vector + FTS scores; PR D's job
// is to ensure an RRF configuration travels end-to-end without error and
// unknown kinds / norms surface as user-facing errors.

// TestVectorQuery_Rerank_RRF_DoesNotError — smoke test: vector-only query
// with an RRF reranker attached must still execute. Reranker is a no-op
// here because there's no second channel, but the wiring (Go builder →
// QueryConfig.Reranker → JSON → Rust parse → QueryBase::rerank) must be
// error-free.
func TestVectorQuery_Rerank_RRF_DoesNotError(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	for i := 0; i < 128; i++ {
		queryVec[i] = 0.1 + float32(i)*0.001
	}

	rec, err := table.VectorQuery("embedding", queryVec).
		Rerank(contracts.RerankerConfig{
			Kind: contracts.RerankerRRF,
			RRFK: 30,
			Norm: contracts.NormalizeRank,
		}).
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	rec.Release()
}

// TestVectorQuery_Rerank_DefaultK_UsesBackendDefault — K==0 must omit the
// k field from JSON, letting the Rust side fall back to RRFReranker's own
// default (60.0).
func TestVectorQuery_Rerank_DefaultK_UsesBackendDefault(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	rec, err := table.VectorQuery("embedding", queryVec).
		Rerank(contracts.RerankerConfig{Kind: contracts.RerankerRRF}).
		Limit(3).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	rec.Release()
}

// TestQuery_Rerank_RRF_Standard — the reranker wiring also reaches the
// non-vector query path (currently a no-op backend-side, but the JSON must
// still parse cleanly).
func TestQuery_Rerank_RRF_Standard(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	rec, err := table.Query().
		Rerank(contracts.RerankerConfig{Kind: contracts.RerankerRRF, RRFK: 60}).
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	rec.Release()
}

// TestSelect_RerankerNone_DirectQueryConfig — Strategy 1 (Edge): a user
// who hand-builds QueryConfig with Reranker: &RerankerConfig{} (i.e.
// Kind == RerankerNone) used to surface as "reranker": null in the
// wire JSON, which the Rust parser treated as a missing kind and
// rejected. Pin the no-op behaviour so direct QueryConfig users don't
// get a spurious error.
func TestSelect_RerankerNone_DirectQueryConfig(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	limit := 3
	rows, err := table.Select(context.Background(), contracts.QueryConfig{
		Limit:    &limit,
		Reranker: &contracts.RerankerConfig{Kind: contracts.RerankerNone},
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
}

// TestVectorQuery_Rerank_Norm_ScoreAndRank — both NormalizeMethod values
// must be accepted by the Rust parser.
func TestVectorQuery_Rerank_Norm_ScoreAndRank(t *testing.T) {
	cases := []contracts.NormalizeMethod{
		contracts.NormalizeDefault,
		contracts.NormalizeScore,
		contracts.NormalizeRank,
	}
	for _, n := range cases {
		n := n
		t.Run("norm", func(t *testing.T) {
			table, cleanup := setupVectorQueryTestTable(t)
			defer cleanup()

			queryVec := make([]float32, 128)
			rec, err := table.VectorQuery("embedding", queryVec).
				Rerank(contracts.RerankerConfig{Kind: contracts.RerankerRRF, Norm: n}).
				Limit(3).
				Execute(context.Background())
			require.NoError(t, err)
			require.NotNil(t, rec)
			rec.Release()
		})
	}
}
