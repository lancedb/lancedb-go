// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// TestSQLDialectColumnVersusLiteral verifies the option-B dialect on a real
// CGO_ENABLED=0 consumer. Double quotes delimit a column identifier; single
// quotes delimit a string literal.
//
// The table deliberately contains a column named "active" and a column named
// "s". `s = "active"` compares two columns, while `s = 'active'` compares the
// column to a string literal. Query, update and delete all use the same
// normalization path, so a silent dialect change can select or affect
// different rows while still reporting success.
func TestSQLDialectColumnVersusLiteral(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "lancedb_sql_dialect_")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(ctx, tempDir, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "s", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "active", Type: arrow.BinaryTypes.String, Nullable: false},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	seed := func(t *testing.T, name string) contracts.ITable {
		t.Helper()
		table, err := conn.CreateTable(ctx, name, schema)
		if err != nil {
			t.Fatalf("create table %s: %v", name, err)
		}
		t.Cleanup(func() { _ = table.Close() })

		pool := memory.NewGoAllocator()
		builder := array.NewRecordBuilder(pool, arrowSchema)
		defer builder.Release()
		builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3}, nil)
		builder.Field(1).(*array.StringBuilder).AppendValues([]string{"active", "inactive", "active"}, nil)
		builder.Field(2).(*array.StringBuilder).AppendValues([]string{"active", "active", "other"}, nil)
		record := builder.NewRecord()
		defer record.Release()
		if err := table.Add(ctx, record, nil); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		return table
	}

	ids := func(t *testing.T, table contracts.ITable, filter string) []int {
		t.Helper()
		rows, err := table.SelectWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("select %q: %v", filter, err)
		}
		out := make([]int, 0, len(rows))
		for _, row := range rows {
			value, ok := row["id"].(float64)
			if !ok {
				t.Fatalf("id value type %T", row["id"])
			}
			out = append(out, int(value))
		}
		sort.Ints(out)
		return out
	}

	columnQuery := seed(t, "sql_column_query")
	literalQuery := seed(t, "sql_literal_query")
	if got, want := ids(t, columnQuery, `s = "active"`), []int{1}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("column comparison selected %v, want %v", got, want)
	}
	if got, want := ids(t, literalQuery, `s = 'active'`), []int{1, 3}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("literal comparison selected %v, want %v", got, want)
	}

	columnUpdate := seed(t, "sql_column_update")
	if err := columnUpdate.Update(ctx, `s = "active"`, map[string]interface{}{"s": "changed"}); err != nil {
		t.Fatalf("column update: %v", err)
	}
	if got, want := ids(t, columnUpdate, `s = 'changed'`), []int{1}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("column update affected %v, want %v", got, want)
	}

	literalUpdate := seed(t, "sql_literal_update")
	if err := literalUpdate.Update(ctx, `s = 'active'`, map[string]interface{}{"s": "changed"}); err != nil {
		t.Fatalf("literal update: %v", err)
	}
	if got, want := ids(t, literalUpdate, `s = 'changed'`), []int{1, 3}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("literal update affected %v, want %v", got, want)
	}

	columnDelete := seed(t, "sql_column_delete")
	if err := columnDelete.Delete(ctx, `s = "active"`); err != nil {
		t.Fatalf("column delete: %v", err)
	}
	if got, want := ids(t, columnDelete, `id >= 0`), []int{2, 3}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("column delete left %v, want %v", got, want)
	}

	literalDelete := seed(t, "sql_literal_delete")
	if err := literalDelete.Delete(ctx, `s = 'active'`); err != nil {
		t.Fatalf("literal delete: %v", err)
	}
	if got, want := ids(t, literalDelete, `id >= 0`), []int{2}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("literal delete left %v, want %v", got, want)
	}
}
