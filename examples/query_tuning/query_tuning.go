// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Per-query tuning example.
//
// Demonstrates the tuning knobs on IVectorQueryBuilder that PR A adds:
//   - Nprobes: IVF partition scan count (recall vs latency)
//   - RefineFactor: IVF refine multiplier
//   - Ef: HNSW candidate list size
//   - BypassVectorIndex: flat scan instead of trained index
//   - Postfilter: run WHERE after candidate set (default is prefilter)
//   - WithRowID: include _rowid column
//   - FastSearch: skip un-indexed rows

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const embeddingDim = 128

func main() {
	fmt.Println("🚀 LanceDB Go SDK - Per-Query Tuning Example")
	fmt.Println("=============================================")

	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "lancedb_query_tuning_example_")
	if err != nil {
		log.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(ctx, tempDir, nil)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(embeddingDim, arrow.PrimitiveTypes.Float32), Nullable: false},
	}, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		log.Fatalf("schema: %v", err)
	}

	table, err := conn.CreateTable(ctx, "docs", schema)
	if err != nil {
		log.Fatalf("create table: %v", err)
	}
	defer table.Close()

	// Seed 200 rows so Nprobes / Ef / BypassVectorIndex have something to
	// operate on. Deterministic per-row vector so the example is repeatable.
	const n = 200
	pool := memory.NewGoAllocator()

	idB := array.NewInt32Builder(pool)
	scoreB := array.NewFloat64Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, embeddingDim, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		scoreB.Append(float64(i) * 0.5)
		embB.Append(true)
		for j := 0; j < embeddingDim; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), scoreB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()

	if err := table.Add(ctx, rec, nil); err != nil {
		log.Fatalf("add: %v", err)
	}

	query := make([]float32, embeddingDim)
	for j := 0; j < embeddingDim; j++ {
		query[j] = 0.5 + float32(j)*0.001
	}

	fmt.Println("\n▶ Baseline: Limit(5)")
	out, err := table.VectorQuery("embedding", query).Limit(5).Execute(ctx)
	if err != nil {
		log.Fatalf("baseline: %v", err)
	}
	fmt.Printf("  returned %d rows, %d columns\n", out.NumRows(), out.NumCols())
	out.Release()

	fmt.Println("\n▶ WithRowID() — result schema includes _rowid")
	out, err = table.VectorQuery("embedding", query).WithRowID().Limit(5).Execute(ctx)
	if err != nil {
		log.Fatalf("with_row_id: %v", err)
	}
	for _, f := range out.Schema().Fields() {
		fmt.Printf("  col: %s\n", f.Name)
	}
	out.Release()

	fmt.Println("\n▶ Postfilter() — WHERE after vector candidate set")
	out, err = table.VectorQuery("embedding", query).
		Filter("score > 50").
		Postfilter().
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("postfilter: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n▶ Tuning combo: Nprobes(10).RefineFactor(2).Ef(64)")
	out, err = table.VectorQuery("embedding", query).
		Nprobes(10).
		RefineFactor(2).
		Ef(64).
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("tuning combo: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n▶ BypassVectorIndex() — flat scan")
	out, err = table.VectorQuery("embedding", query).BypassVectorIndex().Limit(5).Execute(ctx)
	if err != nil {
		log.Fatalf("bypass: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n✅ Per-query tuning example complete")
}
