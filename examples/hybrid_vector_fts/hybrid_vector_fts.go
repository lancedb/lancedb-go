// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Hybrid (vector + FTS) search example.
//
// Demonstrates true hybrid search: a single query that runs both a dense
// vector nearest-neighbour pass and a BM25-style full-text-search pass,
// then fuses the two rankings with RRF (Reciprocal Rank Fusion).
//
// This is distinct from the older "hybrid_search" example in this repo,
// which combines vector search with SQL metadata filtering on the same
// channel. Here we genuinely fuse two score channels.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const dim = 64

func main() {
	fmt.Println("🚀 LanceDB Go SDK - Hybrid (Vector + FTS) Search Example")
	fmt.Println("=========================================================")

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "lancedb_hybrid_vec_fts_example_")
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
		{Name: "body", Type: arrow.BinaryTypes.String, Nullable: false},
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

	docs := []string{
		"the quick brown fox jumps over the lazy dog",
		"a slow blue cat sleeps on the warm mat",
		"fast green turtle swims through the clear river",
		"an orange bird sings in the tall oak tree",
		"the red fox hunts under the moonlit night",
		"lancedb makes fast vector search feel ordinary",
	}

	const n = 200
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	bodyB := array.NewStringBuilder(pool)
	embB := array.NewFixedSizeListBuilder(pool, dim, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		bodyB.Append(docs[i%len(docs)])
		embB.Append(true)
		for j := 0; j < dim; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), bodyB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(ctx, rec, nil); err != nil {
		log.Fatalf("add: %v", err)
	}

	// Hybrid search needs an FTS index on the text column.
	fmt.Println("\n▶ Creating FTS index on body column")
	if err := table.CreateIndexWithParams(
		ctx,
		[]string{"body"},
		contracts.IndexTypeFts,
		contracts.IndexParams{},
		&contracts.CreateIndexOptions{Name: "body_fts", WaitTimeout: 60 * time.Second},
	); err != nil {
		log.Fatalf("create fts: %v", err)
	}
	fmt.Println("  ✓ FTS ready")

	query := make([]float32, dim)
	for j := 0; j < dim; j++ {
		query[j] = 0.5 + float32(j)*0.001
	}

	fmt.Println("\n▶ Hybrid search (vector + FTS, default RRF reranker)")
	out, err := table.VectorQuery("embedding", query).
		WithFullText("brown fox", "body").
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("hybrid: %v", err)
	}
	fmt.Printf("  returned %d rows, columns: ", out.NumRows())
	for _, f := range out.Schema().Fields() {
		fmt.Printf("%s ", f.Name)
	}
	fmt.Println()
	out.Release()

	fmt.Println("\n▶ Hybrid search with explicit RRF (k=30, norm=Rank)")
	out, err = table.VectorQuery("embedding", query).
		WithFullText("green turtle", "body").
		Rerank(contracts.RerankerConfig{
			Kind: contracts.RerankerRRF,
			RRFK: 30,
			Norm: contracts.NormalizeRank,
		}).
		Limit(5).
		Execute(ctx)
	if err != nil {
		log.Fatalf("hybrid rrf: %v", err)
	}
	fmt.Printf("  returned %d rows\n", out.NumRows())
	out.Release()

	fmt.Println("\n✅ Hybrid (vector + FTS) example complete")
}
