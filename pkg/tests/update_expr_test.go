// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// TestUpdateExpr exercises the raw-SQL-expression update path. Each
// sub-test seeds the table with a fixed record set and verifies the
// post-update state via Select queries.
func TestUpdateExpr(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_update_expr_")
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

	seed := func(t *testing.T, name string) contracts.ITable {
		t.Helper()
		table, err := conn.CreateTable(context.Background(), name, schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		t.Cleanup(func() { _ = table.Close() })

		rec := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2, 3, 4, 5},
			[]string{"Alice", "Bob", "Charlie", "Diana", "Eve"},
			[]float64{10, 20, 30, 40, 50})
		defer rec.Release()
		if err := table.Add(context.Background(), rec, nil); err != nil {
			t.Fatalf("seed add: %v", err)
		}
		return table
	}

	t.Run("FilterAndStringLiteral", func(t *testing.T) {
		table := seed(t, "update_expr_filter_literal")
		res, err := table.UpdateExpr(context.Background(), "id > 3",
			[]contracts.UpdateAssignment{
				{Column: "name", Expr: "'updated'"},
			})
		if err != nil {
			t.Fatalf("UpdateExpr: %v", err)
		}
		if res == nil {
			t.Fatal("UpdateExpr returned nil result")
		}
		if res.RowsUpdated != 2 {
			t.Errorf("RowsUpdated = %d, want 2", res.RowsUpdated)
		}
		if res.Version == 0 {
			t.Errorf("Version = 0, want a non-zero post-update version")
		}

		// Verify: ids 4,5 → name="updated"; ids 1,2,3 unchanged.
		rows, err := table.SelectWithFilter(context.Background(), "id > 3")
		if err != nil {
			t.Fatalf("SelectWithFilter: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("post-update row count = %d, want 2", len(rows))
		}
		for _, row := range rows {
			if got, _ := row["name"].(string); got != "updated" {
				t.Errorf("row %v name = %q, want %q", row["id"], got, "updated")
			}
		}
	})

	t.Run("SQLExpressionReadsCurrentValue", func(t *testing.T) {
		// `score = score + 100` — the expression cannot be encoded by the
		// JSON-literal Update path (it would auto-quote to `'score + 100'`
		// and DataFusion would reject the cast). UpdateExpr forwards the
		// expression verbatim so the per-row score doubles-plus-100 lands.
		table := seed(t, "update_expr_sql")
		res, err := table.UpdateExpr(context.Background(), "id <= 3",
			[]contracts.UpdateAssignment{
				{Column: "score", Expr: "score + 100"},
			})
		if err != nil {
			t.Fatalf("UpdateExpr: %v", err)
		}
		if res.RowsUpdated != 3 {
			t.Errorf("RowsUpdated = %d, want 3", res.RowsUpdated)
		}

		rows, err := table.SelectWithFilter(context.Background(), "id <= 3")
		if err != nil {
			t.Fatalf("SelectWithFilter: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("row count = %d, want 3", len(rows))
		}
		want := map[int32]float64{1: 110, 2: 120, 3: 130}
		for _, row := range rows {
			// Numeric columns can come back as int32 / int64 / float64
			// depending on the JSON decoder path — mirror merge_insert_test's
			// shape-tolerant extraction.
			var id int32
			switch v := row["id"].(type) {
			case int32:
				id = v
			case int64:
				id = int32(v)
			case float64:
				id = int32(v)
			default:
				t.Fatalf("unexpected id type %T", row["id"])
			}
			score, _ := row["score"].(float64)
			if score != want[id] {
				t.Errorf("id=%d score = %v, want %v", id, score, want[id])
			}
		}
	})

	t.Run("FunctionCallExpression", func(t *testing.T) {
		// upper(name) — another expression that the JSON-literal Update
		// path cannot represent. Verifies that arbitrary scalar UDFs
		// reach the DataFusion planner intact.
		table := seed(t, "update_expr_func")
		res, err := table.UpdateExpr(context.Background(), "id = 1",
			[]contracts.UpdateAssignment{
				{Column: "name", Expr: "upper(name)"},
			})
		if err != nil {
			t.Fatalf("UpdateExpr: %v", err)
		}
		if res.RowsUpdated != 1 {
			t.Errorf("RowsUpdated = %d, want 1", res.RowsUpdated)
		}

		rows, err := table.SelectWithFilter(context.Background(), "id = 1")
		if err != nil {
			t.Fatalf("SelectWithFilter: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("row count = %d, want 1", len(rows))
		}
		if got, _ := rows[0]["name"].(string); got != "ALICE" {
			t.Errorf("name = %q, want %q", got, "ALICE")
		}
	})

	t.Run("EmptyFilterUpdatesEveryRow", func(t *testing.T) {
		table := seed(t, "update_expr_no_filter")
		res, err := table.UpdateExpr(context.Background(), "",
			[]contracts.UpdateAssignment{
				{Column: "score", Expr: "0"},
			})
		if err != nil {
			t.Fatalf("UpdateExpr: %v", err)
		}
		if res.RowsUpdated != 5 {
			t.Errorf("RowsUpdated = %d, want 5", res.RowsUpdated)
		}

		rows, err := table.SelectWithFilter(context.Background(), "score = 0")
		if err != nil {
			t.Fatalf("SelectWithFilter: %v", err)
		}
		if len(rows) != 5 {
			t.Errorf("zero-score row count = %d, want 5", len(rows))
		}
	})

	t.Run("MultipleAssignmentsPreserveOrder", func(t *testing.T) {
		// Both columns updated in one call. Verifies that the JSON
		// array→Vec path keeps both pairs and applies them in the same
		// commit.
		table := seed(t, "update_expr_multi")
		res, err := table.UpdateExpr(context.Background(), "id = 2",
			[]contracts.UpdateAssignment{
				{Column: "name", Expr: "'multi'"},
				{Column: "score", Expr: "999.5"},
			})
		if err != nil {
			t.Fatalf("UpdateExpr: %v", err)
		}
		if res.RowsUpdated != 1 {
			t.Errorf("RowsUpdated = %d, want 1", res.RowsUpdated)
		}

		rows, err := table.SelectWithFilter(context.Background(), "id = 2")
		if err != nil || len(rows) != 1 {
			t.Fatalf("SelectWithFilter: rows=%d err=%v", len(rows), err)
		}
		if got, _ := rows[0]["name"].(string); got != "multi" {
			t.Errorf("name = %q, want %q", got, "multi")
		}
		if got, _ := rows[0]["score"].(float64); got != 999.5 {
			t.Errorf("score = %v, want 999.5", got)
		}
	})

	t.Run("EmptyAssignmentsRejected", func(t *testing.T) {
		table := seed(t, "update_expr_empty_assign")
		_, err := table.UpdateExpr(context.Background(), "id = 1", nil)
		if err == nil {
			t.Fatal("expected error for empty assignments")
		}
	})

	t.Run("EmptyColumnNameRejected", func(t *testing.T) {
		table := seed(t, "update_expr_empty_col")
		_, err := table.UpdateExpr(context.Background(), "id = 1",
			[]contracts.UpdateAssignment{{Column: "", Expr: "1"}})
		if err == nil {
			t.Fatal("expected error for empty column name")
		}
	})

	t.Run("InvalidFilterReturnsError", func(t *testing.T) {
		table := seed(t, "update_expr_bad_filter")
		_, err := table.UpdateExpr(context.Background(), "this is not sql $$",
			[]contracts.UpdateAssignment{{Column: "score", Expr: "1"}})
		if err == nil {
			t.Fatal("expected error for invalid filter")
		}
	})

	t.Run("InvalidExpressionReturnsError", func(t *testing.T) {
		// Reference a column that does not exist — DataFusion should
		// return a binder error which the FFI surfaces as a normal Go
		// error rather than a panic.
		table := seed(t, "update_expr_bad_expr")
		_, err := table.UpdateExpr(context.Background(), "id = 1",
			[]contracts.UpdateAssignment{{Column: "score", Expr: "no_such_column + 1"}})
		if err == nil {
			t.Fatal("expected error for unknown column reference")
		}
	})

	t.Run("ClosedTableReturnsError", func(t *testing.T) {
		table := seed(t, "update_expr_closed")
		if err := table.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		_, err := table.UpdateExpr(context.Background(), "id = 1",
			[]contracts.UpdateAssignment{{Column: "score", Expr: "1"}})
		if err == nil {
			t.Fatal("expected error for update on closed table")
		}
	})
}
