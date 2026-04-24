// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Index builder example.
//
// Demonstrates CreateIndexWithParams, which exposes the full set of IVF /
// HNSW / FTS tuning parameters plus a name and a replace flag. Compared to
// CreateIndex / CreateIndexWithName (which use backend defaults), this is
// the API to reach for when you want to tune recall / latency or control
// the tokenizer on a full-text index.

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

func u32(v uint32) *uint32 { return &v }
func bptr(v bool) *bool    { return &v }

func main() {
	fmt.Println("🚀 LanceDB Go SDK - Index Builder Example")
	fmt.Println("==========================================")

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "lancedb_index_builder_example_")
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
	bodyB := array.NewStringBuilder(pool)
	embB := array.NewFixedSizeListBuilder(pool, embeddingDim, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		bodyB.Append(fmt.Sprintf("the quick brown fox jumps %d", i))
		embB.Append(true)
		for j := 0; j < embeddingDim; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), bodyB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(ctx, rec, nil); err != nil {
		log.Fatalf("add: %v", err)
	}

	fmt.Println("\n▶ IVF_PQ with custom tuning and a named index")
	if err := table.CreateIndexWithParams(
		ctx,
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		contracts.IndexParams{
			NumPartitions: u32(8),
			NumSubVectors: u32(4),
			NumBits:       u32(8),
			DistanceType:  contracts.DistanceTypeCosine,
		},
		&contracts.CreateIndexOptions{Name: "emb_ivfpq", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatalf("create ivf_pq: %v", err)
	}
	fmt.Println("  ✓ created emb_ivfpq")

	fmt.Println("\n▶ IVF_HNSW_PQ with m and ef_construction (separate name)")
	if err := table.CreateIndexWithParams(
		ctx,
		[]string{"embedding"},
		contracts.IndexTypeHnswPq,
		contracts.IndexParams{
			NumPartitions:  u32(4),
			M:              u32(8),
			EfConstruction: u32(64),
			NumSubVectors:  u32(4),
			DistanceType:   contracts.DistanceTypeL2,
		},
		&contracts.CreateIndexOptions{Name: "emb_hnswpq", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatalf("create hnsw_pq: %v", err)
	}
	fmt.Println("  ✓ created emb_hnswpq")

	fmt.Println("\n▶ FTS with tokenizer options")
	if err := table.CreateIndexWithParams(
		ctx,
		[]string{"body"},
		contracts.IndexTypeFts,
		contracts.IndexParams{
			FtsLanguage:        "English",
			FtsWithPosition:    bptr(true),
			FtsStem:            bptr(true),
			FtsRemoveStopWords: bptr(true),
			FtsLowerCase:       bptr(true),
		},
		&contracts.CreateIndexOptions{Name: "body_fts", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatalf("create fts: %v", err)
	}
	fmt.Println("  ✓ created body_fts")

	indexes, err := table.GetAllIndexes(ctx)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	fmt.Printf("\n📋 Indexes on table: %d\n", len(indexes))
	for _, ix := range indexes {
		fmt.Printf("  - name=%s cols=%v type=%s\n", ix.Name, ix.Columns, ix.IndexType)
	}

	fmt.Println("\n✅ Index builder example complete")
}
