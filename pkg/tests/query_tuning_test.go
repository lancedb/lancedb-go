// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Query tuning tests. These exercise the per-query tuning surface added by
// the PR: nprobes, refine_factor, ef, bypass_vector_index, postfilter,
// with_row_id, fast_search. They share setupVectorQueryTestTable from
// query_builder_test.go.

// TestVectorQuery_WithRowID_AddsRowIDColumn — Strategy 4 (Round Trip):
// when the builder asks for row ids, the returned schema must include the
// _rowid meta column.
func TestVectorQuery_WithRowID_AddsRowIDColumn(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	for i := 0; i < 128; i++ {
		queryVec[i] = 0.1 + float32(i)*0.001
	}

	record, err := table.VectorQuery("embedding", queryVec).
		WithRowID().
		Limit(3).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, record)
	defer record.Release()

	hasRowID := false
	for _, f := range record.Schema().Fields() {
		if f.Name == "_rowid" {
			hasRowID = true
			break
		}
	}
	require.True(t, hasRowID, "expected _rowid column when WithRowID() is set")
}

// TestVectorQuery_TuningParams_DoNotError — Strategy 1 (Edge): confirm every
// tuning knob parses through the JSON config and reaches the Rust FFI without
// erroring on a small synthetic table. Recall differences are infeasible to
// assert on 5 rows — this is the smoke test for the wiring.
func TestVectorQuery_TuningParams_DoNotError(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	for i := 0; i < 128; i++ {
		queryVec[i] = 0.1 + float32(i)*0.001
	}

	cases := []struct {
		name    string
		prepare func(
			ctx context.Context,
			vec []float32,
		) error
	}{
		{
			name: "Nprobes",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).Nprobes(5).Limit(3).Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "RefineFactor",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).RefineFactor(2).Limit(3).Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "Ef",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).Ef(16).Limit(3).Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "BypassVectorIndex",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).BypassVectorIndex().Limit(3).Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "FastSearch",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).FastSearch().Limit(3).Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "Postfilter",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).
					Filter("score > 90").
					Postfilter().
					Limit(3).
					Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
		{
			name: "Combined_NprobesRefineFactorBypass",
			prepare: func(ctx context.Context, vec []float32) error {
				rec, err := table.VectorQuery("embedding", vec).
					Nprobes(5).
					RefineFactor(2).
					BypassVectorIndex().
					Limit(3).
					Execute(ctx)
				if rec != nil {
					defer rec.Release()
				}
				return err
			},
		},
	}

	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NoError(t, c.prepare(ctx, queryVec))
		})
	}
}

// TestQuery_WithRowID_Standard — with_row_id must also flow through the
// standard (non-vector) query path.
func TestQuery_WithRowID_Standard(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	record, err := table.Query().WithRowID().Limit(3).Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, record)
	defer record.Release()

	hasRowID := false
	for _, f := range record.Schema().Fields() {
		if f.Name == "_rowid" {
			hasRowID = true
			break
		}
	}
	require.True(t, hasRowID, "expected _rowid column on standard query when WithRowID() is set")
}

// TestQuery_WithRowID_JSONPath_ReturnsNumericRowID — Strategy 3 (Cross
// Validate): the non-IPC Table.Select path routes through
// convert_arrow_value_to_json, which historically lacked a UInt64 branch
// and emitted the string "Unsupported type: UInt64" for the _rowid
// column. This test pins the numeric representation so a regression
// can't bring the string fallback back.
func TestQuery_WithRowID_JSONPath_ReturnsNumericRowID(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	rows, err := table.Select(context.Background(), contracts.QueryConfig{
		WithRowID: true,
		Limit:     func() *int { n := 3; return &n }(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	for _, r := range rows {
		v, ok := r["_rowid"]
		require.True(t, ok, "_rowid column must be present when WithRowID is set")
		// The JSON path unmarshals numbers into float64; a UInt64 row id
		// must decode to a non-negative number, not a string.
		n, isNumber := v.(float64)
		require.True(t, isNumber, "_rowid must be numeric, got %T (%v)", v, v)
		require.GreaterOrEqual(t, n, 0.0)
	}
}

// TestQuery_FastSearchAndPostfilter_Standard — smoke test the shared
// QueryBase flags on the standard query path.
func TestQuery_FastSearchAndPostfilter_Standard(t *testing.T) {
	table, cleanup := setupQueryTestTable(t)
	defer cleanup()

	rec, err := table.Query().
		Filter("score > 85").
		Postfilter().
		FastSearch().
		Limit(5).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	rec.Release()
}

// TestVectorQuery_ApplyOptions_BypassVectorIndex — ApplyOptions must forward
// BypassVectorIndex through the QueryOptions surface, in addition to
// MaxResults which was already wired.
func TestVectorQuery_ApplyOptions_BypassVectorIndex(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	queryVec := make([]float32, 128)
	opts := &contracts.QueryOptions{MaxResults: 3, BypassVectorIndex: true}

	rec, err := table.VectorQuery("embedding", queryVec).
		ApplyOptions(opts).
		Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	rec.Release()
}
