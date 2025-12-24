package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (contracts.IConnection, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "lancedb_table_test")
	if err != nil {
		t.Fatalf("❌Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.lance")
	conn, err := lancedb.Connect(context.Background(), dbPath, nil)
	if err != nil {
		t.Fatalf("❌Failed to connect to database: %v", err)
	}

	cleanup := func() {
		conn.Close()
		os.RemoveAll(tmpDir)
	}

	return conn, cleanup
}

// createTestTable creates a table with a comprehensive schema for testing
func createTestTable(t *testing.T, conn contracts.IConnection, name string) contracts.ITable {
	t.Helper()

	schema, err := internal.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("name", true).
		AddFloat32Field("score", true).
		AddVectorField("embedding", 128, contracts.VectorDataTypeFloat32, false).
		AddBooleanField("active", true).
		Build()
	if err != nil {
		t.Fatalf("❌Failed to create schema: %v", err)
	}

	table, err := conn.CreateTable(context.Background(), name, schema)
	if err != nil {
		t.Fatalf("❌Failed to create table: %v", err)
	}

	return table
}

func TestTableName(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_table_name")
	defer table.Close()

	if table.Name() != "test_table_name" {
		t.Fatalf("❌Expected table name 'test_table_name', got '%s'", table.Name())
	}
}

func TestTableIsOpen(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_is_open")

	// Table should be open initially
	if !table.IsOpen() {
		t.Fatal("❌Table should be open after creation")
	}

	// Close the table
	err := table.Close()
	if err != nil {
		t.Fatalf("❌Failed to close table: %v", err)
	}

	// Table should be closed now
	if table.IsOpen() {
		t.Fatal("❌Table should be closed after calling Close()")
	}
}

func TestTableSchema(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_schema")
	defer table.Close()

	schema, err := table.Schema(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to get table schema: %v", err)
	}

	// Verify expected fields
	expectedFields := []string{"id", "name", "score", "embedding", "active"}
	if schema.NumFields() != len(expectedFields) {
		t.Fatalf("❌Expected %d fields, got %d", len(expectedFields), schema.NumFields())
	}

	for i, expectedName := range expectedFields {
		field := schema.Field(i)
		if field.Name != expectedName {
			t.Fatalf("❌Expected field %d to be '%s', got '%s'", i, expectedName, field.Name)
		}
	}

	t.Logf("Successfully retrieved schema with %d fields", schema.NumFields())
}

func TestTableCount(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_count")
	defer table.Close()

	// Empty table should have 0 rows
	count, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("❌ Failed to count rows: %v", err)
	}

	if count != 0 {
		t.Fatalf("❌ Expected 0 rows in empty table, got %d", count)
	}

	t.Logf("Table row count: %d", count)
}

func TestTableVersion(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_version")
	defer table.Close()

	version, err := table.Version(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to get table version: %v", err)
	}

	if version < 0 {
		t.Fatalf("❌ Expected non-negative version, got %d", version)
	}

	t.Logf("Table version: %d", version)
}

func TestTableClose(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	table := createTestTable(t, conn, "test_close")

	// Close the table
	err := table.Close()
	if err != nil {
		t.Fatalf("❌Failed to close table: %v", err)
	}

	// Operations on closed table should fail
	_, err = table.Count(context.Background())
	if err == nil {
		t.Fatal("❌Expected error when calling Count() on closed table")
	}

	_, err = table.Schema(context.Background())
	if err == nil {
		t.Fatal("❌Expected error when calling Schema() on closed table")
	}

	_, err = table.Version(context.Background())
	if err == nil {
		t.Fatal("❌Expected error when calling Version() on closed table")
	}
}

func TestOpenTable(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a table
	table1 := createTestTable(t, conn, "test_open_table")
	defer table1.Close()

	// Open the same table with a new handle
	table2, err := conn.OpenTable(context.Background(), "test_open_table")
	if err != nil {
		t.Fatalf("❌Failed to open existing table: %v", err)
	}
	defer table2.Close()

	// Both tables should have the same name
	if table1.Name() != table2.Name() {
		t.Fatalf("❌Table names should match: '%s' vs '%s'", table1.Name(), table2.Name())
	}

	// Both tables should refer to the same data
	count1, err := table1.Count(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to count rows in table1: %v", err)
	}

	count2, err := table2.Count(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to count rows in table2: %v", err)
	}

	if count1 != count2 {
		t.Fatalf("❌Row counts should match: %d vs %d", count1, count2)
	}

	// Both tables should have the same version
	version1, err := table1.Version(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to get version from table1: %v", err)
	}

	version2, err := table2.Version(context.Background())
	if err != nil {
		t.Fatalf("❌Failed to get version from table2: %v", err)
	}

	if version1 != version2 {
		t.Fatalf("❌Versions should match: %d vs %d", version1, version2)
	}
}

func TestTableLifecycle(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	// Create table
	table := createTestTable(t, conn, "test_lifecycle")

	// Test initial state
	if !table.IsOpen() {
		t.Fatal("❌Table should be open after creation")
	}

	if table.Name() != "test_lifecycle" {
		t.Fatalf("❌ Expected table name 'test_lifecycle', got '%s'", table.Name())
	}

	// Test operations work
	count, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("❌ Count should work on open table: %v", err)
	}

	version, err := table.Version(context.Background())
	if err != nil {
		t.Fatalf("❌ Version should work on open table: %v", err)
	}

	schema, err := table.Schema(context.Background())
	if err != nil {
		t.Fatalf("❌ Schema should work on open table: %v", err)
	}

	t.Logf("Table lifecycle test - Count: %d, Version: %d, Fields: %d",
		count, version, schema.NumFields())

	// Close table
	err = table.Close()
	if err != nil {
		t.Fatalf("❌Failed to close table: %v", err)
	}

	// Verify a closed state
	if table.IsOpen() {
		t.Fatal("❌ Table should be closed after Close()")
	}

	// Verify operations fail on a closed table
	_, err = table.Count(context.Background())
	if err == nil {
		t.Fatal("❌ Operations should fail on closed table")
	}
}

func TestAddRecords(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_add_records")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to LanceDB
	db, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to connect to LanceDB: %v", err)
	}
	defer db.Close()

	// Create table schema
	pool := memory.NewGoAllocator()
	arrowSchema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		}, nil,
	)

	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create the table
	table, err := db.CreateTable(context.Background(), "test_add_records", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Create multiple records for batch insertion
	records := make([]arrow.Record, 3)

	// Record 1
	idBuilder1 := array.NewInt64Builder(pool)
	nameBuilder1 := array.NewStringBuilder(pool)
	scoreBuilder1 := array.NewFloat64Builder(pool)

	idBuilder1.AppendValues([]int64{1, 2}, nil)
	nameBuilder1.AppendValues([]string{"Alice", "Bob"}, nil)
	scoreBuilder1.AppendValues([]float64{95.5, 87.3}, nil)

	idArray1 := idBuilder1.NewArray()
	nameArray1 := nameBuilder1.NewArray()
	scoreArray1 := scoreBuilder1.NewArray()

	defer idArray1.Release()
	defer nameArray1.Release()
	defer scoreArray1.Release()

	records[0] = array.NewRecord(arrowSchema, []arrow.Array{idArray1, nameArray1, scoreArray1}, 2)

	// Record 2
	idBuilder2 := array.NewInt64Builder(pool)
	nameBuilder2 := array.NewStringBuilder(pool)
	scoreBuilder2 := array.NewFloat64Builder(pool)

	idBuilder2.AppendValues([]int64{3, 4}, nil)
	nameBuilder2.AppendValues([]string{"Charlie", "Diana"}, nil)
	scoreBuilder2.AppendValues([]float64{92.1, 88.9}, nil)

	idArray2 := idBuilder2.NewArray()
	nameArray2 := nameBuilder2.NewArray()
	scoreArray2 := scoreBuilder2.NewArray()

	defer idArray2.Release()
	defer nameArray2.Release()
	defer scoreArray2.Release()

	records[1] = array.NewRecord(arrowSchema, []arrow.Array{idArray2, nameArray2, scoreArray2}, 2)

	// Record 3
	idBuilder3 := array.NewInt64Builder(pool)
	nameBuilder3 := array.NewStringBuilder(pool)
	scoreBuilder3 := array.NewFloat64Builder(pool)

	idBuilder3.AppendValues([]int64{5}, nil)
	nameBuilder3.AppendValues([]string{"Eve"}, nil)
	scoreBuilder3.AppendValues([]float64{94.7}, nil)

	idArray3 := idBuilder3.NewArray()
	nameArray3 := nameBuilder3.NewArray()
	scoreArray3 := scoreBuilder3.NewArray()

	defer idArray3.Release()
	defer nameArray3.Release()
	defer scoreArray3.Release()

	records[2] = array.NewRecord(arrowSchema, []arrow.Array{idArray3, nameArray3, scoreArray3}, 1)

	// Release records after test
	defer func() {
		for _, record := range records {
			if record != nil {
				record.Release()
			}
		}
	}()

	t.Log("Testing AddRecords method with batch insertion...")
	t.Logf("Adding %d records with total %d rows", len(records),
		records[0].NumRows()+records[1].NumRows()+records[2].NumRows())

	// Test the AddRecords method
	err = table.AddRecords(context.Background(), records, nil)
	if err != nil {
		t.Fatalf("❌Failed to add records: %v", err)
	}

	t.Log("✅Successfully added records to table!")

	// Verify data was added by checking row count
	count, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("Failed to get table count: %v", err)
	}

	expectedCount := int64(5) // 2 + 2 + 1 rows
	if count != expectedCount {
		t.Errorf("Expected %d rows, but got %d", expectedCount, count)
	}

	t.Logf("✅Table now contains %d rows as expected", count)

	// Test with empty records array
	t.Log("Testing AddRecords with empty array...")
	err = table.AddRecords(context.Background(), []arrow.Record{}, nil)
	if err != nil {
		t.Errorf("AddRecords with empty array should not fail: %v", err)
	}

	// Verify count hasn't changed
	newCount, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("Failed to get table count after empty insert: %v", err)
	}

	if newCount != expectedCount {
		t.Errorf("Row count changed after empty insert: expected %d, got %d", expectedCount, newCount)
	}

	t.Log("✅AddRecords with empty array handled correctly")
}

func TestAddRecords_ErrorCases(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_add_records_errors")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to LanceDB
	db, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to connect to LanceDB: %v", err)
	}
	defer db.Close()

	// Create table schema
	pool := memory.NewGoAllocator()
	arrowSchema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		}, nil,
	)

	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create the table
	table, err := db.CreateTable(context.Background(), "test_add_records_errors", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test with closed table
	table.Close()

	// Create a dummy record
	idBuilder := array.NewInt64Builder(pool)
	idBuilder.Append(1)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	record := array.NewRecord(arrowSchema, []arrow.Array{idArray}, 1)
	defer record.Release()

	err = table.AddRecords(context.Background(), []arrow.Record{record}, nil)
	if err == nil {
		t.Error("Expected error when calling AddRecords on closed table")
	}

	if err != nil && err.Error() != "table is closed" {
		t.Errorf("Expected 'table is closed' error, got: %v", err)
	}

	t.Log("✅AddRecords correctly handles closed table")
}

func TestOptimize(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "lancedb_test_optimize")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Connect to LanceDB
	db, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to connect to LanceDB: %v", err)
	}
	defer db.Close()

	// Create table schema
	pool := memory.NewGoAllocator()
	arrowSchema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "vector", Type: arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32)},
			{Name: "value", Type: arrow.BinaryTypes.String},
			{Name: "price", Type: arrow.PrimitiveTypes.Float32},
		}, nil,
	)

	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create the table
	table, err := db.CreateTable(context.Background(), "test", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Create records
	vectorBuilder := array.NewFloat32Builder(pool)
	vectorBuilder.AppendValues([]float32{3.1, 4.1, 5.9, 26.5}, nil)
	vectorFloat32Array := vectorBuilder.NewArray()
	defer vectorFloat32Array.Release()

	vectorListType := arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32)
	vectorArray := array.NewFixedSizeListData(
		array.NewData(vectorListType, 2, []*memory.Buffer{nil},
			[]arrow.ArrayData{vectorFloat32Array.Data()}, 0, 0),
	)
	defer vectorArray.Release()

	itemBuilder := array.NewStringBuilder(pool)
	itemBuilder.AppendValues([]string{"foo", "bar"}, nil)
	itemArray := itemBuilder.NewArray()
	defer itemArray.Release()

	priceBuilder := array.NewFloat32Builder(pool)
	priceBuilder.AppendValues([]float32{10.0, 20.0}, nil)
	priceArray := priceBuilder.NewArray()
	defer priceArray.Release()

	record := array.NewRecord(arrowSchema, []arrow.Array{vectorArray, itemArray, priceArray}, 2)
	defer record.Release()

	err = table.AddRecords(context.Background(), []arrow.Record{record}, nil)
	if err != nil {
		t.Fatalf("Failed to add records: %v", err)
	}

	err = table.CreateIndex(context.Background(), []string{"price"}, contracts.IndexTypeBTree)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	stats, err := table.IndexStats(context.Background(), "price_idx")
	if err != nil {
		t.Fatalf("Failed to get index stats: %v", err)
	}
	if stats.NumIndexedRows != 2 {
		t.Errorf("Expected 2 indexed rows, got %d", stats.NumIndexedRows)
	}

	vectorBuilder2 := array.NewFloat32Builder(pool)
	vectorBuilder2.AppendValues([]float32{2.0, 2.0}, nil)
	vectorFloat32Array2 := vectorBuilder2.NewArray()
	defer vectorFloat32Array2.Release()

	vectorArray2 := array.NewFixedSizeListData(
		array.NewData(vectorListType, 1, []*memory.Buffer{nil},
			[]arrow.ArrayData{vectorFloat32Array2.Data()}, 0, 0),
	)
	defer vectorArray2.Release()

	itemBuilder2 := array.NewStringBuilder(pool)
	itemBuilder2.AppendValues([]string{"baz"}, nil)
	itemArray2 := itemBuilder2.NewArray()
	defer itemArray2.Release()

	priceBuilder2 := array.NewFloat32Builder(pool)
	priceBuilder2.AppendValues([]float32{30.0}, nil)
	priceArray2 := priceBuilder2.NewArray()
	defer priceArray2.Release()

	record2 := array.NewRecord(arrowSchema, []arrow.Array{vectorArray2, itemArray2, priceArray2}, 1)
	defer record2.Release()

	err = table.AddRecords(context.Background(), []arrow.Record{record2}, nil)
	if err != nil {
		t.Fatalf("Failed to add records: %v", err)
	}

	count, err := table.Count(context.Background())
	if err != nil {
		t.Fatalf("Failed to get table count: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 rows in table, got %d", count)
	}

	_, err = table.Optimize(context.Background())
	if err != nil {
		t.Fatalf("Failed to optimize table: %v", err)
	}

	stats, err = table.IndexStats(context.Background(), "price_idx")
	if err != nil {
		t.Fatalf("Failed to get index stats: %v", err)
	}
	if stats.NumIndexedRows != 3 {
		t.Errorf("Expected 2 indexed rows before optimize, got %d", stats.NumIndexedRows)
	}
}

func BenchmarkAddRecords(b *testing.B) {
	// Setup
	tempDir, err := os.MkdirTemp("", "lancedb_bench_add_records")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		b.Fatalf("Failed to connect to LanceDB: %v", err)
	}
	defer db.Close()

	pool := memory.NewGoAllocator()
	arrowSchema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		}, nil,
	)

	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		b.Fatalf("Failed to create schema: %v", err)
	}

	table, err := db.CreateTable(context.Background(), "bench_add_records", schema)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Prepare records for benchmarking
	const batchSize = 100
	records := make([]arrow.Record, 10) // 10 records per batch

	for i := 0; i < 10; i++ {
		idBuilder := array.NewInt64Builder(pool)
		valueBuilder := array.NewFloat64Builder(pool)

		for j := 0; j < batchSize; j++ {
			idBuilder.Append(int64(i*batchSize + j))
			valueBuilder.Append(float64(j) * 1.5)
		}

		idArray := idBuilder.NewArray()
		valueArray := valueBuilder.NewArray()

		records[i] = array.NewRecord(arrowSchema, []arrow.Array{idArray, valueArray}, batchSize)
	}

	// Cleanup
	defer func() {
		for _, record := range records {
			if record != nil {
				record.Release()
			}
		}
	}()

	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		tableName := fmt.Sprintf("bench_table_%d", i)
		testTable, err := db.CreateTable(context.Background(), tableName, schema)
		if err != nil {
			b.Fatalf("Failed to create test table: %v", err)
		}

		err = testTable.AddRecords(context.Background(), records, nil)
		if err != nil {
			testTable.Close()
			b.Fatalf("Failed to add records: %v", err)
		}

		testTable.Close()
	}
}
