// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// TestSchemaEvolve exercises the AddColumns / AlterColumns /
// DropColumns capability extension end-to-end. Each sub-test seeds a
// fresh table so cases stay independent.
func TestSchemaEvolve(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_schema_evolve_")
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

	// seed creates a fresh table named `name`, adds five rows, and
	// returns it asserted to the schema-evolve capability extension.
	seed := func(t *testing.T, name string) (contracts.ITable, contracts.ITableSchemaEvolve) {
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
		se, ok := table.(contracts.ITableSchemaEvolve)
		if !ok {
			t.Fatalf("table does not implement contracts.ITableSchemaEvolve")
		}
		return table, se
	}

	hasField := func(t *testing.T, table contracts.ITable, name string) bool {
		t.Helper()
		s, err := table.Schema(context.Background())
		if err != nil {
			t.Fatalf("Schema: %v", err)
		}
		for _, f := range s.Fields() {
			if f.Name == name {
				return true
			}
		}
		return false
	}

	fieldNullable := func(t *testing.T, table contracts.ITable, name string) bool {
		t.Helper()
		s, err := table.Schema(context.Background())
		if err != nil {
			t.Fatalf("Schema: %v", err)
		}
		for _, f := range s.Fields() {
			if f.Name == name {
				return f.Nullable
			}
		}
		t.Fatalf("field %q not found", name)
		return false
	}

	// Property: AddColumns of a single SQL expression yields a column
	// whose values equal the expression evaluated against existing
	// rows. We seed score = {10..50} and add score_x2 = score * 2,
	// then verify each row.
	t.Run("AddColumnsSqlExpressionProperty", func(t *testing.T) {
		table, se := seed(t, "se_add_sql_expr_property")

		ver, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{
				{Name: "score_x2", Expression: "score * 2"},
			})
		if err != nil {
			t.Fatalf("AddColumns: %v", err)
		}
		if ver == 0 {
			t.Errorf("AddColumns returned version 0, want > 0")
		}
		if !hasField(t, table, "score_x2") {
			t.Fatalf("schema is missing the newly added column score_x2")
		}

		rows, err := table.Select(context.Background(), contracts.QueryConfig{
			Columns: []string{"score", "score_x2"},
		})
		if err != nil {
			t.Fatalf("Select: %v", err)
		}
		if len(rows) != 5 {
			t.Fatalf("Select returned %d rows, want 5", len(rows))
		}
		for i, r := range rows {
			score, ok := r["score"].(float64)
			if !ok {
				t.Fatalf("row %d score has unexpected type %T", i, r["score"])
			}
			x2, ok := r["score_x2"].(float64)
			if !ok {
				t.Fatalf("row %d score_x2 has unexpected type %T", i, r["score_x2"])
			}
			if x2 != score*2 {
				t.Errorf("row %d: score_x2=%v, want %v", i, x2, score*2)
			}
		}
	})

	t.Run("AddColumnsMultipleAtOnce", func(t *testing.T) {
		table, se := seed(t, "se_add_multi")

		_, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{
				{Name: "id_plus_100", Expression: "id + 100"},
				{Name: "score_div_10", Expression: "score / 10"},
			})
		if err != nil {
			t.Fatalf("AddColumns: %v", err)
		}
		if !hasField(t, table, "id_plus_100") || !hasField(t, table, "score_div_10") {
			t.Fatalf("AddColumns did not add both columns")
		}
	})

	t.Run("AddColumnsRejectsBadInput", func(t *testing.T) {
		_, se := seed(t, "se_add_bad_input")
		// Empty slice.
		if _, err := se.AddColumns(context.Background(), nil); err == nil {
			t.Error("expected error for nil transforms")
		}
		if _, err := se.AddColumns(context.Background(), []contracts.NewColumnTransform{}); err == nil {
			t.Error("expected error for empty transforms")
		}
		// Empty Name.
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "", Expression: "score * 2"}}); err == nil {
			t.Error("expected error for empty Name")
		}
		// Whitespace-only Name.
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "   ", Expression: "score * 2"}}); err == nil {
			t.Error("expected error for whitespace-only Name")
		}
		// Empty Expression.
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "x", Expression: ""}}); err == nil {
			t.Error("expected error for empty Expression")
		}
		// Duplicate of an existing column — backend error.
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "id", Expression: "id + 0"}}); err == nil {
			t.Error("expected backend error for duplicate column id")
		}
		// Bad SQL expression — backend error.
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "bad", Expression: "this is not sql !!!"}}); err == nil {
			t.Error("expected backend error for malformed SQL expression")
		}
	})

	// Property: rename preserves data and row count; only the field
	// name changes.
	t.Run("AlterColumnsRename", func(t *testing.T) {
		table, se := seed(t, "se_alter_rename")
		newName := "score_renamed"

		ver, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "score", Rename: &newName}})
		if err != nil {
			t.Fatalf("AlterColumns rename: %v", err)
		}
		if ver == 0 {
			t.Errorf("AlterColumns returned version 0, want > 0")
		}
		if hasField(t, table, "score") {
			t.Errorf("old column 'score' still present after rename")
		}
		if !hasField(t, table, newName) {
			t.Errorf("new column %q not present after rename", newName)
		}
		count, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 5 {
			t.Errorf("row count drifted across rename: got %d, want 5", count)
		}
	})

	t.Run("AlterColumnsNullableToggle", func(t *testing.T) {
		// Loosen nullability (false -> true) on `id`. Lance only
		// allows widening, not tightening — a separate sub-test below
		// verifies that tightening is rejected.
		table, se := seed(t, "se_alter_nullable_loosen")
		if fieldNullable(t, table, "id") {
			t.Fatalf("test precondition: id should start non-nullable")
		}
		nullable := true
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "id", Nullable: &nullable}}); err != nil {
			t.Fatalf("AlterColumns nullable=true: %v", err)
		}
		if !fieldNullable(t, table, "id") {
			t.Errorf("id not nullable after toggle to true")
		}
	})

	t.Run("AlterColumnsNullableTightening", func(t *testing.T) {
		// lancedb v0.24 (lance v1) policy: tightening nullability (true -> false)
		// was always rejected, even when no NULL values were present.
		// lancedb v0.29 (lance v6) policy: permitted when the column has no NULL values.
		// Tightening on empty (or NULL-free) seed data should now succeed.
		_, se := seed(t, "se_alter_nullable_tighten")
		nullable := false
		_, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "score", Nullable: &nullable}})
		if err != nil {
			t.Errorf("expected backend to accept nullable=false on column with no NULL values (lance v6), got: %v", err)
		}
	})

	t.Run("AlterColumnsRejectsBadInput", func(t *testing.T) {
		_, se := seed(t, "se_alter_bad_input")
		// Empty slice.
		if _, err := se.AlterColumns(context.Background(), nil); err == nil {
			t.Error("expected error for nil alterations")
		}
		// Empty path.
		newName := "x"
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "", Rename: &newName}}); err == nil {
			t.Error("expected error for empty Path")
		}
		// No rename and no nullable.
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "score"}}); err == nil {
			t.Error("expected error for alteration with neither rename nor nullable")
		}
		// Empty rename string.
		empty := ""
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "score", Rename: &empty}}); err == nil {
			t.Error("expected error for empty rename string")
		}
		// Unknown column — backend error.
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "ghost", Rename: &newName}}); err == nil {
			t.Error("expected backend error for unknown column")
		}
	})

	// Property: drop removes the column from the schema while leaving
	// row count and other columns untouched.
	t.Run("DropColumnsBasic", func(t *testing.T) {
		table, se := seed(t, "se_drop_basic")

		ver, err := se.DropColumns(context.Background(), []string{"score"})
		if err != nil {
			t.Fatalf("DropColumns: %v", err)
		}
		if ver == 0 {
			t.Errorf("DropColumns returned version 0, want > 0")
		}
		if hasField(t, table, "score") {
			t.Errorf("column 'score' still present after drop")
		}
		if !hasField(t, table, "id") || !hasField(t, table, "name") {
			t.Errorf("DropColumns dropped extra columns: schema=%v",
				schemaFieldNames(t, table))
		}
		count, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 5 {
			t.Errorf("row count drifted across drop: got %d, want 5", count)
		}
	})

	t.Run("DropColumnsMultiple", func(t *testing.T) {
		table, se := seed(t, "se_drop_multi")
		_, err := se.DropColumns(context.Background(), []string{"name", "score"})
		if err != nil {
			t.Fatalf("DropColumns(multi): %v", err)
		}
		if hasField(t, table, "name") || hasField(t, table, "score") {
			t.Errorf("DropColumns(multi) did not remove both columns")
		}
	})

	t.Run("DropColumnsRejectsBadInput", func(t *testing.T) {
		_, se := seed(t, "se_drop_bad_input")
		if _, err := se.DropColumns(context.Background(), nil); err == nil {
			t.Error("expected error for nil names")
		}
		if _, err := se.DropColumns(context.Background(), []string{}); err == nil {
			t.Error("expected error for empty names")
		}
		if _, err := se.DropColumns(context.Background(), []string{""}); err == nil {
			t.Error("expected error for empty name entry")
		}
		if _, err := se.DropColumns(context.Background(), []string{"   "}); err == nil {
			t.Error("expected error for whitespace-only name entry")
		}
		if _, err := se.DropColumns(context.Background(), []string{"ghost"}); err == nil {
			t.Error("expected backend error for unknown column")
		}
	})

	// Round-trip: Add → Alter (rename) → Drop returns the schema to
	// its original field set, and each successful op bumps the table
	// version monotonically.
	t.Run("AddAlterDropRoundTrip", func(t *testing.T) {
		table, se := seed(t, "se_round_trip")

		v0, err := table.Version(context.Background())
		if err != nil {
			t.Fatalf("Version v0: %v", err)
		}

		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "tmp", Expression: "score * 2"}}); err != nil {
			t.Fatalf("AddColumns: %v", err)
		}
		v1, _ := table.Version(context.Background())
		if v1 <= v0 {
			t.Errorf("version did not advance after AddColumns: v0=%d v1=%d", v0, v1)
		}

		newName := "tmp_renamed"
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "tmp", Rename: &newName}}); err != nil {
			t.Fatalf("AlterColumns: %v", err)
		}
		v2, _ := table.Version(context.Background())
		if v2 <= v1 {
			t.Errorf("version did not advance after AlterColumns: v1=%d v2=%d", v1, v2)
		}
		if hasField(t, table, "tmp") || !hasField(t, table, "tmp_renamed") {
			t.Fatalf("rename did not take effect")
		}

		if _, err := se.DropColumns(context.Background(), []string{"tmp_renamed"}); err != nil {
			t.Fatalf("DropColumns: %v", err)
		}
		v3, _ := table.Version(context.Background())
		if v3 <= v2 {
			t.Errorf("version did not advance after DropColumns: v2=%d v3=%d", v2, v3)
		}
		// Schema must match the original {id, name, score}.
		got := schemaFieldNames(t, table)
		want := map[string]bool{"id": true, "name": true, "score": true}
		if len(got) != len(want) {
			t.Fatalf("post-roundtrip field count = %d, want %d (got=%v)",
				len(got), len(want), got)
		}
		for _, n := range got {
			if !want[n] {
				t.Errorf("unexpected field after round-trip: %q", n)
			}
		}
	})

	// Property: dropping a column that has an index also clears that
	// index from the table's index list. Lance handles this in the
	// Operation::Project commit path via retain_relevant_indices —
	// no explicit DropIndex call is required from the caller.
	t.Run("DropColumnsCascadesToIndex", func(t *testing.T) {
		table, se := seed(t, "se_drop_cascades_index")

		// Build a scalar index on `score`; BTree is the cheapest to
		// build on a 5-row table.
		if err := table.CreateIndex(context.Background(),
			[]string{"score"}, contracts.IndexTypeBTree); err != nil {
			t.Fatalf("CreateIndex(BTree, score): %v", err)
		}
		// Wait for the index to be ready before checking GetAllIndexes
		// — index build is async on the backend.
		if err := table.WaitForIndex(context.Background(), nil, 30*time.Second); err != nil {
			t.Fatalf("WaitForIndex: %v", err)
		}

		before, err := table.GetAllIndexes(context.Background())
		if err != nil {
			t.Fatalf("GetAllIndexes (before drop): %v", err)
		}
		var hadScoreIdx bool
		for _, ix := range before {
			for _, c := range ix.Columns {
				if c == "score" {
					hadScoreIdx = true
				}
			}
		}
		if !hadScoreIdx {
			t.Fatalf("test precondition failed: no index on 'score' before drop. Indexes=%+v", before)
		}

		if _, err := se.DropColumns(context.Background(), []string{"score"}); err != nil {
			t.Fatalf("DropColumns(score): %v", err)
		}

		after, err := table.GetAllIndexes(context.Background())
		if err != nil {
			t.Fatalf("GetAllIndexes (after drop): %v", err)
		}
		for _, ix := range after {
			for _, c := range ix.Columns {
				if c == "score" {
					t.Errorf("index on dropped column 'score' still present: %+v", ix)
				}
			}
		}
	})

	t.Run("ClosedTableReturnsError", func(t *testing.T) {
		table, se := seed(t, "se_closed")
		if err := table.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if _, err := se.AddColumns(context.Background(),
			[]contracts.NewColumnTransform{{Name: "x", Expression: "1"}}); err == nil {
			t.Error("expected AddColumns on closed table to fail")
		}
		newName := "x"
		if _, err := se.AlterColumns(context.Background(),
			[]contracts.ColumnAlteration{{Path: "score", Rename: &newName}}); err == nil {
			t.Error("expected AlterColumns on closed table to fail")
		}
		if _, err := se.DropColumns(context.Background(), []string{"score"}); err == nil {
			t.Error("expected DropColumns on closed table to fail")
		}
	})
}

func schemaFieldNames(t *testing.T, table contracts.ITable) []string {
	t.Helper()
	s, err := table.Schema(context.Background())
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	out := make([]string, 0, len(s.Fields()))
	for _, f := range s.Fields() {
		out = append(out, f.Name)
	}
	return out
}
