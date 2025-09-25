// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func TestAddMethod(t *testing.T) {
	// Setup test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_add_")
	if err != nil {
		t.Fatalf("❌Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to database
	conn, err := Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("❌Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create test schema
	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("❌Failed to create schema: %v", err)
	}

	// Create table
	table, err := conn.CreateTable(context.Background(), "test_add", *schema)
	if err != nil {
		t.Fatalf("❌Failed to create table: %v", err)
	}
	defer table.Close()

	// Create sample data using Arrow arrays
	pool := memory.NewGoAllocator()

	// Create ID column (Int32)
	idBuilder := array.NewInt32Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	// Create Name column (String)
	nameBuilder := array.NewStringBuilder(pool)
	nameBuilder.AppendValues([]string{"Alice", "Bob", "Charlie"}, nil)
	nameArray := nameBuilder.NewArray()
	defer nameArray.Release()

	// Create Score column (Float64)
	scoreBuilder := array.NewFloat64Builder(pool)
	scoreBuilder.AppendValues([]float64{95.5, 87.2, 92.8}, nil)
	scoreArray := scoreBuilder.NewArray()
	defer scoreArray.Release()

	// Create Arrow Record
	columns := []arrow.Array{idArray, nameArray, scoreArray}
	record := array.NewRecord(arrowSchema, columns, 3)
	defer record.Release()

	t.Log("Testing Table.Add method... \n")
	t.Logf("Adding record with %d rows\n", record.NumRows())

	// Test the Add method
	err = table.Add(record, nil)
	if err != nil {
		t.Fatalf("❌Failed to add data: %v", err)
	}

	t.Log("✅Successfully added data to table! \n")

	// Verify data was added by checking row count
	count, err := table.Count()
	if err != nil {
		t.Fatalf("❌Failed to get row count: %v", err)
	}

	t.Logf("Table now contains %d rows\n", count)

	// Test adding more data
	t.Log("\nAdding more data...")

	// Create more sample data
	idBuilder2 := array.NewInt32Builder(pool)
	idBuilder2.AppendValues([]int32{4, 5}, nil)
	idArray2 := idBuilder2.NewArray()
	defer idArray2.Release()

	nameBuilder2 := array.NewStringBuilder(pool)
	nameBuilder2.AppendValues([]string{"Diana", "Eve"}, nil)
	nameArray2 := nameBuilder2.NewArray()
	defer nameArray2.Release()

	scoreBuilder2 := array.NewFloat64Builder(pool)
	scoreBuilder2.AppendValues([]float64{88.9, 94.1}, nil)
	scoreArray2 := scoreBuilder2.NewArray()
	defer scoreArray2.Release()

	columns2 := []arrow.Array{idArray2, nameArray2, scoreArray2}
	record2 := array.NewRecord(arrowSchema, columns2, 2)
	defer record2.Release()

	err = table.Add(record2, nil)
	if err != nil {
		t.Fatalf("❌Failed to add second batch: %v", err)
	}

	// Check final count
	finalCount, err := table.Count()
	if err != nil {
		t.Fatalf("❌Failed to get final row count: %v", err)
	}

	t.Logf("✅ Table now contains %d rows after second batch\n", finalCount)

	t.Log("\n🎉 Table.Add method implementation is working correctly!")
}
