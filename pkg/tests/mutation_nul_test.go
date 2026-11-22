// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// TestMutationRejectsEmbeddedNUL covers every Go string that reaches a DML
// statement as an identifier or predicate. A NUL would terminate the C string
// before the intended value, so these calls must fail before native code and
// leave the table unchanged.
func TestMutationRejectsEmbeddedNUL(t *testing.T) {
	table, cleanup := newLegacyUpdateTable(t)
	defer cleanup()
	ctx := context.Background()
	before, err := table.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	requireNUL := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s succeeded", name)
		}
		if !strings.Contains(err.Error(), "embedded NUL") {
			t.Fatalf("%s error = %v, want an embedded NUL rejection", name, err)
		}
	}

	requireNUL("update predicate", table.Update(ctx, "id = 1\x00 OR 1=1", map[string]interface{}{"text": "changed"}))
	requireNUL("update column", table.Update(ctx, "id = 1", map[string]interface{}{"text\x00shadow": "changed"}))
	requireNUL("update value", table.Update(ctx, "id = 1", map[string]interface{}{"text": "changed\x00tail"}))
	requireNUL("delete predicate", table.Delete(ctx, "id = 1\x00 OR 1=1"))

	updates, ok := table.(contracts.ITableUpdateExpr)
	if !ok {
		t.Fatal("table does not expose ITableUpdateExpr")
	}
	requireNUL("update_expr predicate", mustUpdateExpr(updates, ctx, "id = 1\x00", []contracts.UpdateAssignment{{Column: "text", Expr: "'x'"}}))
	requireNUL("update_expr column", mustUpdateExpr(updates, ctx, "", []contracts.UpdateAssignment{{Column: "text\x00shadow", Expr: "'x'"}}))
	requireNUL("update_expr expression", mustUpdateExpr(updates, ctx, "", []contracts.UpdateAssignment{{Column: "text", Expr: "'x'\x00"}}))

	concrete := asMergeInsertTable(t, table)
	requireNUL("merge_insert on", mergeError(concrete.MergeInsert([]string{"id\x00"}).WhenNotMatchedInsertAll(), ctx))
	condition := "id = 1\x00"
	requireNUL("merge_insert condition", mergeError(concrete.MergeInsert([]string{"id"}).WhenMatchedUpdateAll(&condition).WhenNotMatchedInsertAll(), ctx))
	filter := "id = 1\x00"
	requireNUL("merge_insert filter", mergeError(concrete.MergeInsert([]string{"id"}).WhenNotMatchedBySourceDelete(&filter), ctx))

	after, err := table.Version(ctx)
	if err != nil {
		t.Fatalf("version after failed mutations: %v", err)
	}
	if after != before {
		t.Fatalf("failed mutations advanced version from %d to %d", before, after)
	}
	for id, want := range map[int64]string{1: "one", 2: "two", 3: "three"} {
		if got, _ := rowText(t, table, id); got != want {
			t.Fatalf("row %d text = %q, want %q", id, got, want)
		}
	}
}

func mustUpdateExpr(updates contracts.ITableUpdateExpr, ctx context.Context, filter string, assignments []contracts.UpdateAssignment) error {
	_, err := updates.UpdateExpr(ctx, filter, assignments)
	return err
}

func mergeError(builder contracts.IMergeInsertBuilder, ctx context.Context) error {
	_, err := builder.Execute(ctx, nil)
	return err
}

func TestTableNamesRejectEmbeddedNUL(t *testing.T) {
	ctx := context.Background()
	conn, err := lancedb.Connect(ctx, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()
	schema, err := internal.NewSchema(arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
	}, nil))
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	name := "items\x00shadow"
	for label, call := range map[string]func() error{
		"create": func() error {
			_, err := conn.CreateTable(ctx, name, schema)
			return err
		},
		"open": func() error {
			_, err := conn.OpenTable(ctx, name)
			return err
		},
		"drop": func() error {
			return conn.DropTable(ctx, name)
		},
	} {
		err := call()
		if err == nil || !strings.Contains(err.Error(), "embedded NUL") {
			t.Fatalf("%s table name error = %v, want an embedded NUL rejection", label, err)
		}
	}
	names, err := conn.TableNames(ctx)
	if err != nil {
		t.Fatalf("table names: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("NUL-named table was created: %v", names)
	}
}
