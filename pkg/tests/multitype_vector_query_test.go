// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// setupF64EmbeddingTable seeds a small table whose `embedding` column is
// FixedSizeList<Float64> rather than the default Float32. Used to verify
// VectorQueryF64 against a column whose dtype actually matches.
func setupF64EmbeddingTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_vqf64_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(8, arrow.PrimitiveTypes.Float64), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), "vqf64", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 16
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, 8, arrow.PrimitiveTypes.Float64)
	embValB := embB.ValueBuilder().(*array.Float64Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		embB.Append(true)
		for j := 0; j < 8; j++ {
			embValB.Append(float64(i)*0.1 + float64(j)*0.01)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(context.Background(), rec, nil); err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("add: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// TestVectorQueryF64_MatchingColumn — happy path: query an f64 column
// with an f64 query vector. Rust dispatches Vec<f64> → IntoQueryVector
// → nearest_to; no cast required. Also exercises the optional
// capability interface contract (ITableMultiDtypeVectorQuery), which
// is the supported way for downstream callers to discover the new
// dtype constructors without adding methods to ITable.
func TestVectorQueryF64_MatchingColumn(t *testing.T) {
	table, cleanup := setupF64EmbeddingTable(t)
	defer cleanup()

	var iface contracts.ITable = table
	mq, ok := iface.(contracts.ITableMultiDtypeVectorQuery)
	require.True(t, ok, "*internal.Table must implement ITableMultiDtypeVectorQuery")

	q := make([]float64, 8)
	for i := range q {
		q[i] = 0.05
	}
	rec, err := mq.VectorQueryF64("embedding", q).Limit(3).Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()
	require.Equal(t, int64(3), rec.NumRows(), "expected K=3 rows from f64 vector query")
}

// TestVectorQueryF64_CastsToF32Column — lance's IntoQueryVector for
// Vec<f64> will cast down to f32 when the column is FixedSizeList<f32>.
// Reuse the shared f32 setup from query_builder_test.go and confirm
// the cast path returns rows instead of erroring out.
func TestVectorQueryF64_CastsToF32Column(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	q := make([]float64, 128)
	for i := range q {
		q[i] = 0.1 + float64(i)*0.001
	}
	rec, err := table.VectorQueryF64("embedding", q).Limit(2).Execute(context.Background())
	require.NoError(t, err, "f64 query against an f32 column should cast cleanly")
	require.NotNil(t, rec)
	defer rec.Release()
	require.Equal(t, int64(2), rec.NumRows())
}

// f16BitsFromF32 converts an IEEE 754 single-precision value into the
// raw bits of its half-precision equivalent (round-to-nearest-even,
// no subnormal flushing). Reimplemented here so the test file stays
// dependency-free; callers in production code should use a tested
// half-precision library.
func f16BitsFromF32(f float32) uint16 {
	x := math.Float32bits(f)
	sign := uint16((x >> 16) & 0x8000)
	mant := x & 0x007fffff
	exp := int32((x>>23)&0xff) - 127 + 15
	switch {
	case exp >= 0x1f:
		return sign | 0x7c00 // inf
	case exp <= 0:
		if exp < -10 {
			return sign
		}
		mant |= 0x00800000
		shift := uint32(14 - exp)
		return sign | uint16(mant>>shift)
	default:
		return sign | uint16(exp<<10) | uint16(mant>>13)
	}
}

// TestVectorQueryF16_CastsToF32Column — pass an f16 query vector (raw
// bits) against an f32 column. lance's IntoQueryVector for Vec<f16>
// casts up to f32 on the column-dtype mismatch, so the query must
// succeed without erroring.
func TestVectorQueryF16_CastsToF32Column(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	bits := make([]uint16, 128)
	for i := range bits {
		bits[i] = f16BitsFromF32(0.1 + float32(i)*0.001)
	}
	rec, err := table.VectorQueryF16("embedding", bits).Limit(2).Execute(context.Background())
	require.NoError(t, err, "f16 query against an f32 column should cast cleanly")
	require.NotNil(t, rec)
	defer rec.Release()
	require.Equal(t, int64(2), rec.NumRows())
}

// TestVectorQuery_EmptyVector_RejectedByAllDtypes — the existing f32
// guard is extended to f64, f16, and u8: an empty input slice must be
// rejected before the FFI call, with a consistent error message.
func TestVectorQuery_EmptyVector_RejectedByAllDtypes(t *testing.T) {
	table, cleanup := setupVectorQueryTestTable(t)
	defer cleanup()

	cases := []struct {
		name string
		run  func() error
	}{
		{"f32", func() error {
			_, e := table.VectorQuery("embedding", []float32{}).Limit(1).Execute(context.Background())
			return e
		}},
		{"f64", func() error {
			_, e := table.VectorQueryF64("embedding", []float64{}).Limit(1).Execute(context.Background())
			return e
		}},
		{"f16", func() error {
			_, e := table.VectorQueryF16("embedding", []uint16{}).Limit(1).Execute(context.Background())
			return e
		}},
		{"u8", func() error {
			_, e := table.VectorQueryU8("embedding", []uint8{}).Limit(1).Execute(context.Background())
			return e
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.run()
			require.Error(t, err, "%s: empty vector must be rejected", c.name)
			require.Contains(t, err.Error(), "non-empty",
				"%s: error message should reference non-empty contract; got: %v", c.name, err)
		})
	}
}

// setupU8EmbeddingTable seeds a small table whose `embedding` column is
// FixedSizeList<UInt8>. Used to verify VectorQueryU8 against a column
// whose dtype actually matches.
func setupU8EmbeddingTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "lancedb_test_vqu8_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(8, arrow.PrimitiveTypes.Uint8), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), "vqu8", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 16
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, 8, arrow.PrimitiveTypes.Uint8)
	embValB := embB.ValueBuilder().(*array.Uint8Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		embB.Append(true)
		for j := 0; j < 8; j++ {
			embValB.Append(uint8((i*8 + j) & 0xff))
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(context.Background(), rec, nil); err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("add: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// TestVectorQueryU8_MatchingColumn — happy path: query a UInt8 column
// with a u8 query vector. Rust constructs a 1-D Arrow UInt8 array and
// routes it through lancedb's IntoQueryVector for Arc<dyn Array>. Also
// exercises the capability-interface contract for ITableUint8VectorQuery
// (mirroring the F64 test's discovery pattern).
func TestVectorQueryU8_MatchingColumn(t *testing.T) {
	table, cleanup := setupU8EmbeddingTable(t)
	defer cleanup()

	var iface contracts.ITable = table
	uq, ok := iface.(contracts.ITableUint8VectorQuery)
	require.True(t, ok, "*internal.Table must implement ITableUint8VectorQuery")

	q := []uint8{10, 20, 30, 40, 50, 60, 70, 80}
	rec, err := uq.VectorQueryU8("embedding", q).Limit(3).Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rec)
	defer rec.Release()
	require.Equal(t, int64(3), rec.NumRows(), "expected K=3 rows from u8 vector query")
}

// TestVectorQueryU8_BoundaryValues — Strategy 1 (Edge): probe the 0 and
// 255 boundary values that exercise the u8 range without exceeding it.
// The Rust side validates the JSON wire numbers are 0..=255, so both
// extremes must round-trip cleanly.
func TestVectorQueryU8_BoundaryValues(t *testing.T) {
	table, cleanup := setupU8EmbeddingTable(t)
	defer cleanup()

	cases := []struct {
		name string
		vec  []uint8
	}{
		{"all_zero", []uint8{0, 0, 0, 0, 0, 0, 0, 0}},
		{"all_max", []uint8{255, 255, 255, 255, 255, 255, 255, 255}},
		{"mixed", []uint8{0, 255, 1, 254, 127, 128, 63, 192}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, err := table.VectorQueryU8("embedding", c.vec).Limit(1).
				Execute(context.Background())
			require.NoError(t, err)
			require.NotNil(t, rec)
			defer rec.Release()
			require.Equal(t, int64(1), rec.NumRows())
		})
	}
}
