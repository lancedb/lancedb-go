// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// WaitForIndex example.
//
// Demonstrates how to block until an index finishes building before
// issuing the first query against it. Useful for boot sequences that
// want predictable latency on the very first search.

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

const embeddingDim = 64

func main() {
	fmt.Println("🚀 LanceDB Go SDK - WaitForIndex Example")
	fmt.Println("=========================================")

	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "lancedb_wait_for_index_example_")
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

	const n = 300
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	embB := array.NewFixedSizeListBuilder(pool, embeddingDim, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		embB.Append(true)
		for j := 0; j < embeddingDim; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(ctx, rec, nil); err != nil {
		log.Fatalf("add: %v", err)
	}

	fmt.Println("\n▶ CreateIndex (returns before the index is fully built)")
	if err := table.CreateIndexWithName(ctx, []string{"embedding"}, contracts.IndexTypeIvfPq, "emb_idx"); err != nil {
		log.Fatalf("create index: %v", err)
	}

	fmt.Println("▶ WaitForIndex timeout=30s")
	start := time.Now()
	if err := table.WaitForIndex(ctx, []string{"emb_idx"}, 30*time.Second); err != nil {
		log.Fatalf("wait: %v", err)
	}
	fmt.Printf("  done after %s\n", time.Since(start).Round(time.Millisecond))

	stats, err := table.IndexStats(ctx, "emb_idx")
	if err != nil {
		log.Fatalf("stats: %v", err)
	}
	fmt.Printf("  indexed=%d unindexed=%d\n", stats.NumIndexedRows, stats.NumUnindexedRows)

	fmt.Println("\n✅ WaitForIndex example complete")
}
