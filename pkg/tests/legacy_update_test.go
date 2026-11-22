// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

type namedLabel string

type customString struct{}

func (customString) MarshalJSON() ([]byte, error) { return []byte(`"custom-value"`), nil }

type invalidJSON struct{}

func (invalidJSON) MarshalJSON() ([]byte, error) { return []byte(`id + 3`), nil }

func newLegacyUpdateTable(t *testing.T) (contracts.ITable, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_legacy_update_")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}
	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}, Nullable: false},
		{Name: "nullable", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "f64", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "legacy_update", schema)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowSchema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2, 3}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"one", "two", "three"}, nil)
	ts := builder.Field(2).(*array.TimestampBuilder)
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts.AppendTime(stamp)
	ts.AppendTime(stamp.Add(time.Hour))
	ts.AppendTime(stamp.Add(2 * time.Hour))
	builder.Field(3).(*array.StringBuilder).AppendNull()
	builder.Field(3).(*array.StringBuilder).Append("present")
	builder.Field(3).(*array.StringBuilder).AppendNull()
	builder.Field(4).(*array.Float64Builder).AppendValues([]float64{1, 2, 3}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(context.Background(), record, nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	return table, func() {
		_ = table.Close()
		_ = conn.Close()
		_ = os.RemoveAll(tempDir)
	}
}

func rowText(t *testing.T, table contracts.ITable, id int64) (string, bool) {
	t.Helper()
	rows, err := table.SelectWithFilter(context.Background(), "id = "+strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("select id %d: %v", id, err)
	}
	if len(rows) != 1 {
		t.Fatalf("select id %d returned %d rows", id, len(rows))
	}
	text, ok := rows[0]["text"].(string)
	return text, ok
}

func TestLegacyUpdatePredicateAndJSONScalars(t *testing.T) {
	table, cleanup := newLegacyUpdateTable(t)
	defer cleanup()
	ctx := context.Background()

	before, err := table.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if err := table.Update(ctx, "", map[string]interface{}{"text": "whole-table"}); err == nil {
		t.Fatal("empty predicate update succeeded")
	}
	if err := table.Update(ctx, "   ", map[string]interface{}{"text": "whole-table"}); err == nil {
		t.Fatal("whitespace predicate update succeeded")
	}
	if got, _ := rowText(t, table, 1); got != "one" {
		t.Fatalf("empty predicate changed row: %q", got)
	}

	if err := table.Update(ctx, "id = 1", map[string]interface{}{"text": namedLabel("named")}); err != nil {
		t.Fatalf("named string update: %v", err)
	}
	if got, _ := rowText(t, table, 1); got != "named" {
		t.Fatalf("named string update wrote %q", got)
	}

	stamp := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := table.Update(ctx, "id = 1", map[string]interface{}{"ts": stamp}); err != nil {
		t.Fatalf("time.Time update: %v", err)
	}
	if err := table.Update(ctx, "id = 2", map[string]interface{}{"nullable": (*string)(nil)}); err != nil {
		t.Fatalf("typed nil update: %v", err)
	}
	if err := table.Update(ctx, "id = 3", map[string]interface{}{"text": customString{}}); err != nil {
		t.Fatalf("custom marshaller update: %v", err)
	}
	if got, _ := rowText(t, table, 3); got != "custom-value" {
		t.Fatalf("custom marshaller update wrote %q", got)
	}

	versionAfterSuccess, err := table.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	for _, value := range []interface{}{
		math.NaN(),
		json.Number("id + 3"),
		[]string{"composite"},
		invalidJSON{},
	} {
		if err := table.Update(ctx, "id = 1", map[string]interface{}{"f64": value}); err == nil {
			t.Fatalf("update with %T(%v) succeeded", value, value)
		}
	}
	versionAfterFailures, err := table.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if versionAfterFailures != versionAfterSuccess {
		t.Fatalf("failed updates advanced version from %d to %d", versionAfterSuccess, versionAfterFailures)
	}
	rows, err := table.SelectWithFilter(ctx, "id >= 0")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("failed updates changed row count: %d", len(rows))
	}
	if got, _ := rowText(t, table, 1); got != "named" {
		t.Fatalf("failed update changed text: %q", got)
	}
	if after, err := table.Version(ctx); err != nil || after != versionAfterSuccess {
		t.Fatalf("successful updates should be the only version changes: %d -> %d, %v", before, after, err)
	}
}
