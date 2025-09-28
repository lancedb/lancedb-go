// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../../lib/darwin_amd64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../../lib/darwin_arm64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../../lib/linux_amd64/liblancedb_go.a -lm -ldl -lpthread
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/../../lib/linux_arm64/liblancedb_go.a -lm -ldl -lpthread
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/../../lib/windows_amd64/liblancedb_go.a
#include "lancedb.h"
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Table represents a table in the LanceDB database
type Table struct {
	name       string
	connection *Connection
	// #nosec G103 - FFI handle for C interop with Rust library
	handle unsafe.Pointer
	mu     sync.RWMutex
	closed bool
}

// Compile-time check to ensure Table implements ITable interface
var _ contracts.ITable = (*Table)(nil)

// Name returns the name of the Table
func (t *Table) Name() string {
	return t.name
}

// IsOpen returns true if the Table is still open
func (t *Table) IsOpen() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed && !t.connection.closed
}

// Close closes the Table and releases resources
func (t *Table) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.handle == nil {
		return nil
	}

	result := C.simple_lancedb_table_close(t.handle)
	defer C.simple_lancedb_result_free(result)

	t.handle = nil
	t.closed = true
	runtime.SetFinalizer(t, nil)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to close table: %s", errorMsg)
		}
		return fmt.Errorf("failed to close table: unknown error")
	}

	return nil
}

// Schema returns the schema of the Table using efficient Arrow IPC format
//
//nolint:gocritic
func (t *Table) Schema(_ context.Context) (*arrow.Schema, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	var schemaIPCData *C.uchar
	var schemaIPCLen C.size_t
	result := C.simple_lancedb_table_schema_ipc(t.handle, &schemaIPCData, &schemaIPCLen)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to get table schema: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to get table schema: unknown error")
	}

	if schemaIPCData == nil {
		return nil, fmt.Errorf("received null schema IPC data")
	}

	// Free the IPC data when we're done
	defer C.simple_lancedb_free_ipc_data(schemaIPCData)

	// Convert C data to Go slice
	// #nosec G103 - Safe conversion of C memory to Go bytes for Arrow IPC data
	ipcBytes := C.GoBytes(unsafe.Pointer(schemaIPCData), C.int(schemaIPCLen))

	// Create a reader from the IPC bytes
	reader, err := ipc.NewFileReader(bytes.NewReader(ipcBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create IPC reader: %w", err)
	}
	defer reader.Close()

	// Get the schema from the IPC reader
	schema := reader.Schema()
	if schema == nil {
		return nil, fmt.Errorf("failed to read schema from IPC data")
	}

	return schema, nil
}

// Add inserts data into the Table
func (t *Table) Add(ctx context.Context, record arrow.Record, _ *contracts.AddDataOptions) error {
	var r []arrow.Record
	if record != nil {
		r = append(r, record)
	}
	return t.AddRecords(ctx, r, nil)
}

// AddRecords efficiently adds multiple records using Arrow IPC batch processing
func (t *Table) AddRecords(_ context.Context, records []arrow.Record, _ *contracts.AddDataOptions) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	if len(records) == 0 {
		return nil
	}

	// Convert records to Arrow RecordBatch using Arrow IPC format
	var buf bytes.Buffer
	seeker := &seekBuffer{&buf}
	writer, err := ipc.NewFileWriter(seeker, ipc.WithSchema(records[0].Schema()))
	if err != nil {
		return fmt.Errorf("failed to create IPC writer: %w", err)
	}

	for _, record := range records {
		if err := writer.Write(record); err != nil {
			err := writer.Close()
			if err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}
			return fmt.Errorf("failed to write record to IPC: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close IPC writer: %w", err)
	}

	// Get the IPC bytes
	ipcBytes := buf.Bytes()
	if len(ipcBytes) == 0 {
		return fmt.Errorf("no IPC data generated")
	}

	// Call the Rust function with IPC binary data
	var addedCount C.int64_t
	result := C.simple_lancedb_table_add_ipc(
		t.handle,
		// #nosec G103 - Safe conversion of Go slice to C array pointer for FFI
		(*C.uchar)(unsafe.Pointer(&ipcBytes[0])),
		C.ulong(len(ipcBytes)),
		&addedCount,
	)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to add records: %s", errorMsg)
		}
		return fmt.Errorf("failed to add records: unknown error")
	}

	return nil
}

// seekBuffer wraps a bytes.Buffer to implement io.WriteSeeker
type seekBuffer struct {
	*bytes.Buffer
}

func (sb *seekBuffer) Seek(offset int64, whence int) (int64, error) {
	// For simplicity, we only support seeking to the end for append operations
	switch whence {
	case 2: // io.SeekEnd
		return int64(sb.Len()), nil
	case 0: // io.SeekStart
		if offset == 0 {
			sb.Reset()
			return 0, nil
		}
		return 0, fmt.Errorf("seeking to non-zero position not supported")
	case 1: // io.SeekCurrent
		return int64(sb.Len()), nil
	default:
		return 0, fmt.Errorf("unsupported whence value")
	}
}

// Query creates a new query builder for this Table
func (t *Table) Query() contracts.IQueryBuilder {
	return &QueryBuilder{
		table:   t,
		filters: make([]string, 0),
		limit:   -1,
	}
}

// Count returns the number of rows in the Table
func (t *Table) Count(_ context.Context) (int64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}

	var count C.int64_t
	result := C.simple_lancedb_table_count_rows(t.handle, &count)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return 0, fmt.Errorf("failed to count rows: %s", errorMsg)
		}
		return 0, fmt.Errorf("failed to count rows: unknown error")
	}

	return int64(count), nil
}

// Version returns the current version of the Table
func (t *Table) Version(_ context.Context) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}

	var version C.int64_t
	result := C.simple_lancedb_table_version(t.handle, &version)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return 0, fmt.Errorf("failed to get table version: %s", errorMsg)
		}
		return 0, fmt.Errorf("failed to get table version: unknown error")
	}

	return int(version), nil
}

// Update updates records in the Table based on a filter
func (t *Table) Update(_ context.Context, filter string, updates map[string]interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	// Convert updates map to JSON
	updatesJSON, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal updates to JSON: %w", err)
	}

	cFilter := C.CString(filter)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cFilter))

	cUpdatesJSON := C.CString(string(updatesJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cUpdatesJSON))

	result := C.simple_lancedb_table_update(t.handle, cFilter, cUpdatesJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to update rows: %s", errorMsg)
		}
		return fmt.Errorf("failed to update rows: unknown error")
	}

	return nil
}

// Delete deletes records from the Table based on a filter
func (t *Table) Delete(_ context.Context, filter string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	cFilter := C.CString(filter)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cFilter))

	var deletedCount C.int64_t
	result := C.simple_lancedb_table_delete(t.handle, cFilter, &deletedCount)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to delete rows: %s", errorMsg)
		}
		return fmt.Errorf("failed to delete rows: unknown error")
	}

	// Note: deletedCount is set to -1 in the Rust implementation since LanceDB doesn't expose the count
	// We could return the count if needed, but for now we just ensure the operation succeeded
	return nil
}

// CreateIndex creates an index on the specified columns
func (t *Table) CreateIndex(ctx context.Context, columns []string, indexType contracts.IndexType) error {
	return t.CreateIndexWithName(ctx, columns, indexType, "")
}

// CreateIndexWithName creates an index on the specified columns with an optional name
func (t *Table) CreateIndexWithName(_ context.Context, columns []string, indexType contracts.IndexType, name string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	if len(columns) == 0 {
		return fmt.Errorf("columns list cannot be empty")
	}

	// Convert columns to JSON
	columnsJSON, err := json.Marshal(columns)
	if err != nil {
		return fmt.Errorf("failed to marshal columns to JSON: %w", err)
	}

	// Convert index type to string
	indexTypeStr := t.indexTypeToString(indexType)

	cColumnsJSON := C.CString(string(columnsJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cColumnsJSON))

	cIndexType := C.CString(indexTypeStr)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cIndexType))

	var cIndexName *C.char
	if name != "" {
		cIndexName = C.CString(name)
		// #nosec G103 - Required for freeing C allocated string memory
		defer C.free(unsafe.Pointer(cIndexName))
	}

	result := C.simple_lancedb_table_create_index(t.handle, cColumnsJSON, cIndexType, cIndexName)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to create index: %s", errorMsg)
		}
		return fmt.Errorf("failed to create index: unknown error")
	}

	return nil
}

// GetAllIndexes returns information about all indexes created on this table
//
//nolint:gocritic
func (t *Table) GetAllIndexes(_ context.Context) ([]contracts.IndexInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	var indexesJSON *C.char
	result := C.simple_lancedb_table_get_indexes(t.handle, &indexesJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to get indexes: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to get indexes: unknown error")
	}

	if indexesJSON == nil {
		return []contracts.IndexInfo{}, nil // Return empty slice if no indexes
	}

	jsonStr := C.GoString(indexesJSON)
	C.simple_lancedb_free_string(indexesJSON)

	// Parse JSON response
	var indexes []contracts.IndexInfo
	if err := json.Unmarshal([]byte(jsonStr), &indexes); err != nil {
		return nil, fmt.Errorf("failed to parse indexes JSON: %w", err)
	}

	return indexes, nil
}

// Select executes a select query with various predicates (vector search, filters, etc.)
//
//nolint:gocritic
func (t *Table) Select(_ context.Context, config contracts.QueryConfig) ([]map[string]interface{}, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	// Convert lancedb.QueryConfig to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query config to JSON: %w", err)
	}

	cConfigJSON := C.CString(string(configJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cConfigJSON))

	var resultJSON *C.char
	result := C.simple_lancedb_table_select_query(t.handle, cConfigJSON, &resultJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to execute select query: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to execute select query: unknown error")
	}

	if resultJSON == nil {
		return []map[string]interface{}{}, nil // Return empty slice if no results
	}

	jsonStr := C.GoString(resultJSON)
	C.simple_lancedb_free_string(resultJSON)

	// Parse JSON response
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rows); err != nil {
		return nil, fmt.Errorf("failed to parse query results JSON: %w", err)
	}

	return rows, nil
}

// SelectWithColumns is a convenience method for selecting specific columns
func (t *Table) SelectWithColumns(ctx context.Context, columns []string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		Columns: columns,
	})
}

// SelectWithFilter is a convenience method for selecting with a WHERE filter
func (t *Table) SelectWithFilter(ctx context.Context, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		Where: filter,
	})
}

// VectorSearch is a convenience method for vector similarity search
func (t *Table) VectorSearch(ctx context.Context, column string, vector []float32, k int) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		VectorSearch: &contracts.VectorSearch{
			Column: column,
			Vector: vector,
			K:      k,
		},
	})
}

// VectorSearchWithFilter combines vector search with additional filtering
func (t *Table) VectorSearchWithFilter(ctx context.Context, column string, vector []float32, k int, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		VectorSearch: &contracts.VectorSearch{
			Column: column,
			Vector: vector,
			K:      k,
		},
		Where: filter,
	})
}

// FullTextSearch is a convenience method for full-text search
func (t *Table) FullTextSearch(ctx context.Context, column string, query string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		FTSSearch: &contracts.FTSSearch{
			Column: column,
			Query:  query,
		},
	})
}

// FullTextSearchWithFilter combines full-text search with additional filtering
func (t *Table) FullTextSearchWithFilter(ctx context.Context, column string, query string, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		FTSSearch: &contracts.FTSSearch{
			Column: column,
			Query:  query,
		},
		Where: filter,
	})
}

// SelectWithLimit is a convenience method for selecting with limit and offset
func (t *Table) SelectWithLimit(ctx context.Context, limit int, offset int) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{
		Limit:  &limit,
		Offset: &offset,
	})
}

// indexTypeToString converts IndexType enum to string representation
func (t *Table) indexTypeToString(indexType contracts.IndexType) string {
	switch indexType {
	case contracts.IndexTypeAuto:
		return "vector" // Default to vector index for auto
	case contracts.IndexTypeIvfPq:
		return "ivf_pq"
	case contracts.IndexTypeIvfFlat:
		return "ivf_flat"
	case contracts.IndexTypeHnswPq:
		return "hnsw_pq"
	case contracts.IndexTypeHnswSq:
		return "hnsw_sq"
	case contracts.IndexTypeBTree:
		return "btree"
	case contracts.IndexTypeBitmap:
		return "bitmap"
	case contracts.IndexTypeLabelList:
		return "label_list"
	case contracts.IndexTypeFts:
		return "fts"
	default:
		return "vector" // Default fallback
	}
}

// recordToJSON converts an Arrow Record to JSON format
func (t *Table) recordToJSON(record arrow.Record) (string, error) {
	schema := record.Schema()
	rows := make([]map[string]interface{}, record.NumRows())

	// Initialize rows
	for i := range rows {
		rows[i] = make(map[string]interface{})
	}

	// Process each column
	for colIdx, field := range schema.Fields() {
		column := record.Column(colIdx)
		fieldName := field.Name

		if err := t.convertColumnToJSON(column, fieldName, field.Type, rows); err != nil {
			return "", fmt.Errorf("failed to convert column %s: %w", fieldName, err)
		}
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// convertColumnToJSON converts an Arrow column to JSON values in the rows
//
//nolint:gocyclo,nestif, gocognit
func (t *Table) convertColumnToJSON(column arrow.Array, fieldName string, dataType arrow.DataType, rows []map[string]interface{}) error {
	switch dataType.ID() {
	case arrow.INT32:
		arr := column.(*array.Int32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.INT64:
		arr := column.(*array.Int64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.FLOAT32:
		arr := column.(*array.Float32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.FLOAT64:
		arr := column.(*array.Float64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.BOOL:
		arr := column.(*array.Boolean)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.STRING:
		arr := column.(*array.String)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsNull(i) {
				rows[i][fieldName] = nil
			} else {
				rows[i][fieldName] = arr.Value(i)
			}
		}
	case arrow.FIXED_SIZE_LIST:
		arr := column.(*array.FixedSizeList)
		listType := dataType.(*arrow.FixedSizeListType)

		// Handle vector fields (FixedSizeList of Float32)
		if listType.Elem().ID() == arrow.FLOAT32 {
			for i := 0; i < arr.Len(); i++ {
				if arr.IsNull(i) {
					rows[i][fieldName] = nil
				} else {
					listStart := i * int(listType.Len())
					values := make([]float32, listType.Len())

					valueArray := arr.ListValues().(*array.Float32)
					for j := 0; j < int(listType.Len()); j++ {
						if listStart+j < valueArray.Len() {
							values[j] = valueArray.Value(listStart + j)
						}
					}
					rows[i][fieldName] = values
				}
			}
		} else {
			return fmt.Errorf("unsupported FixedSizeList element type: %s", listType.Elem())
		}
	default:
		return fmt.Errorf("unsupported Arrow type: %s", dataType)
	}

	return nil
}
