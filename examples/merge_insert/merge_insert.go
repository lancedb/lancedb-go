// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

// Merge Insert (Upsert) Example
//
// This example demonstrates the MergeInsert builder for atomic upserts:
// - Insert-only: insert rows that do not already exist on the key column
// - Upsert: update matched rows and insert unmatched ones in a single call
// - Conditional update: only replace matched rows that satisfy an SQL predicate
//   comparing target.* (existing row) with source.* (incoming row)

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

func main() {
	fmt.Println("🚀 LanceDB Go SDK - Merge Insert (Upsert) Example")
	fmt.Println("==================================================")

	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "lancedb_merge_insert_example_")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(ctx, tempDir, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	schema, err := lancedb.NewSchema(arrowSchema)
	if err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	table, err := conn.CreateTable(ctx, "users", schema)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	pool := memory.NewGoAllocator()

	// --- Seed: three initial rows -------------------------------------
	seed := makeRecord(pool, arrowSchema,
		[]int32{1, 2, 3},
		[]string{"Alice", "Bob", "Charlie"},
		[]float64{50, 50, 50})
	defer seed.Release()
	if err := table.Add(ctx, seed, nil); err != nil {
		log.Fatalf("Failed to seed rows: %v", err)
	}
	printCount(ctx, table, "After seed")

	// --- Scenario 1: upsert (match-update + not-matched-insert) --------
	//
	// ids 2 and 3 overlap the existing rows → they are replaced.
	// id 4 is new → it is inserted.
	upsert := makeRecord(pool, arrowSchema,
		[]int32{2, 3, 4},
		[]string{"Bob v2", "Charlie v2", "Diana"},
		[]float64{222, 333, 444})
	defer upsert.Release()

	res, err := table.
		MergeInsert([]string{"id"}).
		WhenMatchedUpdateAll(nil).
		WhenNotMatchedInsertAll().
		Execute(ctx, []arrow.Record{upsert})
	if err != nil {
		log.Fatalf("Upsert failed: %v", err)
	}
	fmt.Printf("\n✨ Upsert result: inserted=%d updated=%d deleted=%d version=%d\n",
		res.NumInsertedRows, res.NumUpdatedRows, res.NumDeletedRows, res.Version)
	printCount(ctx, table, "After upsert")

	// --- Scenario 2: conditional update -------------------------------
	//
	// Only replace a matched row when the incoming score is higher than
	// the existing score. Reference the two sides as `target.*` / `source.*`.
	candidate := makeRecord(pool, arrowSchema,
		[]int32{1, 2},
		[]string{"Alice new", "Bob new"},
		[]float64{99 /* > 50, updates */, 10 /* < 222, skipped */})
	defer candidate.Release()

	cond := "target.score < source.score"
	res, err = table.
		MergeInsert([]string{"id"}).
		WhenMatchedUpdateAll(&cond).
		Execute(ctx, []arrow.Record{candidate})
	if err != nil {
		log.Fatalf("Conditional upsert failed: %v", err)
	}
	fmt.Printf("\n✨ Conditional update result: updated=%d (rows whose target.score < source.score)\n",
		res.NumUpdatedRows)

	rows, err := table.SelectWithFilter(ctx, "id IN (1, 2)")
	if err != nil {
		log.Fatalf("Select failed: %v", err)
	}
	fmt.Println("\nFinal state of ids 1 and 2:")
	for _, r := range rows {
		fmt.Printf("  id=%v  name=%q  score=%v\n", r["id"], r["name"], r["score"])
	}
}

func makeRecord(pool memory.Allocator, s *arrow.Schema, ids []int32, names []string, scores []float64) arrow.Record {
	idB := array.NewInt32Builder(pool)
	idB.AppendValues(ids, nil)
	idArr := idB.NewArray()
	defer idArr.Release()

	nameB := array.NewStringBuilder(pool)
	nameB.AppendValues(names, nil)
	nameArr := nameB.NewArray()
	defer nameArr.Release()

	scoreB := array.NewFloat64Builder(pool)
	scoreB.AppendValues(scores, nil)
	scoreArr := scoreB.NewArray()
	defer scoreArr.Release()

	return array.NewRecord(s, []arrow.Array{idArr, nameArr, scoreArr}, int64(len(ids)))
}

func printCount(ctx context.Context, table interface {
	Count(context.Context) (int64, error)
}, label string) {
	n, err := table.Count(ctx)
	if err != nil {
		log.Fatalf("%s: Count failed: %v", label, err)
	}
	fmt.Printf("%s: %d rows\n", label, n)
}
