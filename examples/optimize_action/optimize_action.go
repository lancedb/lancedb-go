// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// OptimizeWithAction example.
//
// Demonstrates the configurable optimize sub-actions:
//   - OptimizeCompact: merge small fragments
//   - OptimizePrune:   reclaim disk taken by old versions
//   - OptimizeIndex:   fold unindexed rows into existing indices
//   - OptimizeAll:     run everything (default)

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

func u64(v uint64) *uint64 { return &v }
func bp(v bool) *bool      { return &v }

func main() {
	fmt.Println("🚀 LanceDB Go SDK - OptimizeWithAction Example")
	fmt.Println("===============================================")

	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "lancedb_optimize_action_example_")
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
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
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

	// Three small Add calls => three fragments worth compacting.
	pool := memory.NewGoAllocator()
	for batch := 0; batch < 3; batch++ {
		idB := array.NewInt32Builder(pool)
		nB := array.NewStringBuilder(pool)
		for i := 0; i < 10; i++ {
			idB.Append(int32(batch*10 + i))
			nB.Append(fmt.Sprintf("row-%d", batch*10+i))
		}
		rec := array.NewRecord(arrowSchema,
			[]arrow.Array{idB.NewArray(), nB.NewArray()}, 10)
		if err := table.Add(ctx, rec, nil); err != nil {
			rec.Release()
			log.Fatalf("add: %v", err)
		}
		rec.Release()
	}

	fmt.Println("\n▶ Compact (TargetRowsPerFragment=100)")
	stats, err := table.OptimizeWithAction(ctx, contracts.OptimizeAction{
		Kind: contracts.OptimizeCompact,
		Compaction: contracts.CompactionParams{
			TargetRowsPerFragment: u64(100),
		},
	})
	if err != nil {
		log.Fatalf("compact: %v", err)
	}
	if stats.Compaction != nil {
		fmt.Printf("  fragments_removed=%v, fragments_added=%v, files_removed=%v, files_added=%v\n",
			ptrOrZero(stats.Compaction.FragmentsRemoved),
			ptrOrZero(stats.Compaction.FragmentsAdded),
			ptrOrZero(stats.Compaction.FilesRemoved),
			ptrOrZero(stats.Compaction.FilesAdded),
		)
	}

	fmt.Println("\n▶ Prune (OlderThan=1h, DeleteUnverified=true)")
	if _, err := table.OptimizeWithAction(ctx, contracts.OptimizeAction{
		Kind: contracts.OptimizePrune,
		Prune: contracts.PruneParams{
			OlderThan:        time.Hour,
			DeleteUnverified: bp(true),
		},
	}); err != nil {
		log.Fatalf("prune: %v", err)
	}
	fmt.Println("  ✓ prune ok")

	fmt.Println("\n▶ Index (no-op when no index exists)")
	if _, err := table.OptimizeWithAction(ctx, contracts.OptimizeAction{
		Kind: contracts.OptimizeIndex,
	}); err != nil {
		log.Fatalf("index: %v", err)
	}
	fmt.Println("  ✓ index ok")

	fmt.Println("\n▶ All (legacy Optimize entry point)")
	if _, err := table.Optimize(ctx); err != nil {
		log.Fatalf("all: %v", err)
	}
	fmt.Println("  ✓ all ok")

	fmt.Println("\n✅ OptimizeWithAction example complete")
}

func ptrOrZero(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
