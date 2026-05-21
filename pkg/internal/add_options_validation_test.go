// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/require"
)

func setupAddOptionsTable(t *testing.T) (*Table, []byte, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_test_add_opts_")
	require.NoError(t, err)

	conn, err := openSimpleConnection(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}
	tbl, err := conn.CreateTable(context.Background(), "add_opts", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create: %v", err)
	}

	pool := memory.NewGoAllocator()
	b := array.NewInt32Builder(pool)
	b.AppendValues([]int32{1, 2, 3, 4}, nil)
	col := b.NewArray()
	rec := array.NewRecord(arrowSchema, []arrow.Array{col}, 4)
	defer rec.Release()
	ipc, err := recordsToIPCBytes([]arrow.Record{rec})
	require.NoError(t, err)

	cleanup := func() {
		tbl.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return tbl.(*Table), ipc, cleanup
}

// openSimpleConnection is a thin wrapper over the internal-package
// Connect (introduced for tests so we can avoid a pkg/lancedb import
// cycle).
func openSimpleConnection(uri string) (*Connection, error) {
	return Connect(context.Background(), uri)
}

// TestAddIPCWithOptions_InvalidParallelismTypes — the strict validator
// in the Rust FFI must reject write_parallelism values that aren't
// non-negative integers. Silent fallback to default would hide caller
// misconfiguration. Exercises three concrete bad shapes that the typed
// *AddDataOptions Go API cannot construct.
func TestAddIPCWithOptions_InvalidParallelismTypes(t *testing.T) {
	table, ipc, cleanup := setupAddOptionsTable(t)
	defer cleanup()

	cases := []struct {
		name string
		json string
	}{
		{"string", `{"write_parallelism":"4"}`},
		{"negative", `{"write_parallelism":-1}`},
		{"float", `{"write_parallelism":1.5}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := table.addIPCWithRawOptions(ipc, c.json)
			require.Error(t, err, "%s: must reject invalid type", c.name)
			require.Contains(t, strings.ToLower(err.Error()), "write_parallelism",
				"%s: error should name the field; got: %v", c.name, err)
		})
	}
}

// TestAddIPCWithOptions_NullParallelism — explicit null is documented as
// "use the lancedb default" and must succeed.
func TestAddIPCWithOptions_NullParallelism(t *testing.T) {
	table, ipc, cleanup := setupAddOptionsTable(t)
	defer cleanup()
	require.NoError(t,
		table.addIPCWithRawOptions(ipc, `{"write_parallelism":null}`))
}

// TestAddIPCWithOptions_UnknownKeys — unknown keys must be silently
// ignored so future knobs can be added additively.
func TestAddIPCWithOptions_UnknownKeys(t *testing.T) {
	table, ipc, cleanup := setupAddOptionsTable(t)
	defer cleanup()
	require.NoError(t,
		table.addIPCWithRawOptions(ipc, `{"future_unknown_knob":true}`))
}

// TestAddIPCWithOptions_NonObjectPayload — the FFI contract says
// options_json is a JSON object. Top-level arrays, scalars, strings,
// and bools must error out instead of silently falling through to
// "no options", which would hide caller serialization bugs.
func TestAddIPCWithOptions_NonObjectPayload(t *testing.T) {
	table, ipc, cleanup := setupAddOptionsTable(t)
	defer cleanup()

	cases := []struct {
		name string
		json string
	}{
		{"array", `[]`},
		{"number", `42`},
		{"string", `"foo"`},
		{"bool", `true`},
		{"null", `null`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := table.addIPCWithRawOptions(ipc, c.json)
			require.Error(t, err, "%s: non-object payload must be rejected", c.name)
			require.Contains(t, err.Error(), "object",
				"%s: error should mention 'object' contract; got: %v", c.name, err)
		})
	}
}
