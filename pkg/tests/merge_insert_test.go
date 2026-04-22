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

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func TestMergeInsert(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_merge_insert_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	pool := memory.NewGoAllocator()

	t.Run("InsertOnly", func(t *testing.T) {
		table, err := conn.CreateTable(context.Background(), "merge_insert_only", schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		defer table.Close()

		seed := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2, 3},
			[]string{"Alice", "Bob", "Charlie"},
			[]float64{10, 20, 30})
		defer seed.Release()
		if err := table.Add(context.Background(), seed, nil); err != nil {
			t.Fatalf("seed add: %v", err)
		}

		fresh := buildRecord(t, pool, arrowSchema,
			[]int32{4, 5, 6},
			[]string{"Diana", "Eve", "Frank"},
			[]float64{40, 50, 60})
		defer fresh.Release()

		res, err := table.MergeInsert([]string{"id"}).
			WhenNotMatchedInsertAll().
			Execute(context.Background(), []arrow.Record{fresh})
		if err != nil {
			t.Fatalf("merge_insert failed: %v", err)
		}
		if res.NumInsertedRows != 3 {
			t.Errorf("NumInsertedRows = %d, want 3", res.NumInsertedRows)
		}
		if res.NumUpdatedRows != 0 {
			t.Errorf("NumUpdatedRows = %d, want 0", res.NumUpdatedRows)
		}
		if got, _ := table.Count(context.Background()); got != 6 {
			t.Errorf("Count = %d, want 6", got)
		}
	})

	t.Run("UpsertMatchAndInsert", func(t *testing.T) {
		table, err := conn.CreateTable(context.Background(), "merge_upsert", schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		defer table.Close()

		seed := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2, 3},
			[]string{"Alice", "Bob", "Charlie"},
			[]float64{10, 20, 30})
		defer seed.Release()
		if err := table.Add(context.Background(), seed, nil); err != nil {
			t.Fatalf("seed add: %v", err)
		}

		// ids 2 and 3 overlap the seed; id 4 is new.
		upsert := buildRecord(t, pool, arrowSchema,
			[]int32{2, 3, 4},
			[]string{"Bob-v2", "Charlie-v2", "Diana"},
			[]float64{222, 333, 444})
		defer upsert.Release()

		res, err := table.MergeInsert([]string{"id"}).
			WhenMatchedUpdateAll(nil).
			WhenNotMatchedInsertAll().
			Execute(context.Background(), []arrow.Record{upsert})
		if err != nil {
			t.Fatalf("merge_insert failed: %v", err)
		}
		if res.NumUpdatedRows != 2 {
			t.Errorf("NumUpdatedRows = %d, want 2", res.NumUpdatedRows)
		}
		if res.NumInsertedRows != 1 {
			t.Errorf("NumInsertedRows = %d, want 1", res.NumInsertedRows)
		}
		if got, _ := table.Count(context.Background()); got != 4 {
			t.Errorf("Count = %d, want 4", got)
		}

		// Verify id=2 was actually replaced.
		rows, err := table.SelectWithFilter(context.Background(), "id = 2")
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("select id=2 rows = %d, want 1", len(rows))
		}
		if name, _ := rows[0]["name"].(string); name != "Bob-v2" {
			t.Errorf("id=2 name = %q, want %q", name, "Bob-v2")
		}
	})

	t.Run("ConditionalUpdate", func(t *testing.T) {
		// Only update matched rows where the source score is higher.
		table, err := conn.CreateTable(context.Background(), "merge_conditional", schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		defer table.Close()

		seed := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2},
			[]string{"Alice", "Bob"},
			[]float64{50, 50})
		defer seed.Release()
		if err := table.Add(context.Background(), seed, nil); err != nil {
			t.Fatalf("seed add: %v", err)
		}

		// id=1 has a higher source score (should update), id=2 has a lower one (should NOT update).
		src := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2},
			[]string{"Alice-new", "Bob-new"},
			[]float64{99, 10})
		defer src.Release()

		cond := "target.score < source.score"
		res, err := table.MergeInsert([]string{"id"}).
			WhenMatchedUpdateAll(&cond).
			Execute(context.Background(), []arrow.Record{src})
		if err != nil {
			t.Fatalf("merge_insert failed: %v", err)
		}
		if res.NumUpdatedRows != 1 {
			t.Errorf("NumUpdatedRows = %d, want 1", res.NumUpdatedRows)
		}

		rows, err := table.Select(context.Background(), contracts.QueryConfig{})
		if err != nil {
			t.Fatalf("select: %v", err)
		}
		byID := map[int32]map[string]interface{}{}
		for _, r := range rows {
			// Count comes back as float64 when decoded from JSON.
			switch v := r["id"].(type) {
			case float64:
				byID[int32(v)] = r
			case int32:
				byID[v] = r
			case int64:
				byID[int32(v)] = r
			}
		}
		if name, _ := byID[1]["name"].(string); name != "Alice-new" {
			t.Errorf("id=1 name = %q, want %q (should be updated)", name, "Alice-new")
		}
		if name, _ := byID[2]["name"].(string); name != "Bob" {
			t.Errorf("id=2 name = %q, want %q (should be unchanged)", name, "Bob")
		}
	})

	t.Run("ErrorPaths", func(t *testing.T) {
		table, err := conn.CreateTable(context.Background(), "merge_errors", schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}

		rec := buildRecord(t, pool, arrowSchema,
			[]int32{1}, []string{"x"}, []float64{1})
		defer rec.Release()

		// Empty `on` must fail before the FFI call.
		if _, err := table.MergeInsert(nil).
			WhenNotMatchedInsertAll().
			Execute(context.Background(), []arrow.Record{rec}); err == nil {
			t.Error("expected error for empty on")
		}

		// No actions configured is an error.
		if _, err := table.MergeInsert([]string{"id"}).
			Execute(context.Background(), []arrow.Record{rec}); err == nil {
			t.Error("expected error when no actions configured")
		}

		// Closed table.
		table.Close()
		if _, err := table.MergeInsert([]string{"id"}).
			WhenNotMatchedInsertAll().
			Execute(context.Background(), []arrow.Record{rec}); err == nil {
			t.Error("expected error on closed table")
		}
	})

	t.Run("RejectsMalformedConfig", func(t *testing.T) {
		// A malformed condition type on the Rust-side config must surface as
		// an error rather than be silently dropped to "no condition" (which
		// would broaden a matched-update or by-source-delete).
		table, err := conn.CreateTable(context.Background(), "merge_malformed", schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		defer table.Close()

		// The public Go API only accepts *string here, so we can't construct
		// a numeric condition through it — but we can at least exercise a
		// known-bad filter-by-source on an empty source to confirm the
		// Rust-side null-pointer guard engages cleanly (bad-SQL → clean err).
		badFilter := "this is not valid SQL )))"
		if _, err := table.MergeInsert([]string{"id"}).
			WhenNotMatchedBySourceDelete(&badFilter).
			Execute(context.Background(), nil); err == nil {
			t.Error("expected error for invalid SQL in by-source filter")
		}
	})
}

func buildRecord(t *testing.T, pool memory.Allocator, s *arrow.Schema, ids []int32, names []string, scores []float64) arrow.Record {
	t.Helper()
	idB := array.NewInt32Builder(pool)
	idB.AppendValues(ids, nil)
	idArr := idB.NewArray()
	defer idArr.Release()

	nameB := array.NewStringBuilder(pool)
	nameB.AppendValues(names, nil)
	nameArr := nameB.NewArray()
	defer nameArr.Release()

	scoreB := array.NewFloat64Builder(pool)
	scoreB.AppendValues(scores, nil)
	scoreArr := scoreB.NewArray()
	defer scoreArr.Release()

	return array.NewRecord(s, []arrow.Array{idArr, nameArr, scoreArr}, int64(len(ids)))
}
