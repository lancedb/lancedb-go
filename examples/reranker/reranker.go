// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Reranker (RRF) example.
//
// Demonstrates attaching an RRF reranker to a query. Reranking only has
// observable effect when the query has two score channels to fuse — the
// hybrid search example (examples/hybrid_search) shows that combination.
// This example focuses on the API surface: how to pick a k value and a
// normalization method.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const dim = 64

func main() {
	fmt.Println("🚀 LanceDB Go SDK - Reranker (RRF) Example")
	fmt.Println("===========================================")

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "lancedb_reranker_example_")
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
		{Name: "embedding", Type: arrow.FixedSizeListOf(dim, arrow.PrimitiveTypes.Float32), Nullable: false},
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

	const n = 100
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, dim, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		embB.Append(true)
		for j := 0; j < dim; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(ctx, rec, nil); err != nil {
		log.Fatalf("add: %v", err)
	}

	queryVec := make([]float32, dim)
	for j := 0; j < dim; j++ {
		queryVec[j] = 0.5 + float32(j)*0.001
	}

	fmt.Println("\n▶ Vector query + RRF reranker (k=60, norm=Rank)")
	out, err := table.VectorQuery("embedding", queryVec).
		Rerank(contracts.RerankerConfig{
			Kind: contracts.RerankerRRF,
			RRFK: 60,
			Norm: contracts.NormalizeRank,
		}).
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n▶ Default K (backend default = 60.0)")
	out, err = table.VectorQuery("embedding", queryVec).
		Rerank(contracts.RerankerConfig{Kind: contracts.RerankerRRF}).
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("query default k: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n✅ Reranker example complete")
	fmt.Println("   Try combining with hybrid search (vector + FTS) to see")
	fmt.Println("   actual rerank behaviour across two score channels.")
}
