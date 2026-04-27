// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#include "lancedb.h"
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
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

	ipcBytes, err := recordsToIPCBytes(records)
	if err != nil {
		return err
	}
	if len(ipcBytes) == 0 {
		return fmt.Errorf("no IPC data generated")
	}

	// Call the Rust function with IPC binary data
	var addedCount C.int64_t
	result := C.simple_lancedb_table_add_ipc(
		t.handle,
		// #nosec G103 - Safe conversion of Go slice to C array pointer for FFI
		(*C.uchar)(unsafe.Pointer(&ipcBytes[0])),
		C.size_t(uintptr(len(ipcBytes))),
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

func (t *Table) MergeInsert(on []string) contracts.IMergeInsertBuilder {
	onCopy := append([]string(nil), on...)
	return &MergeInsertBuilder{table: t, on: onCopy}
}

// Query creates a new query builder for this Table
func (t *Table) Query() contracts.IQueryBuilder {
	return &QueryBuilder{
		table:   t,
		filters: make([]string, 0),
	}
}

// VectorQuery creates a new vector query builder for this Table
func (t *Table) VectorQuery(column string, vector []float32) contracts.IVectorQueryBuilder {
	var vectorCopy []float32
	if vector != nil {
		vectorCopy = make([]float32, len(vector))
		copy(vectorCopy, vector)
	}
	return &VectorQueryBuilder{
		QueryBuilder: QueryBuilder{
			table:   t,
			filters: make([]string, 0),
			columns: nil,
		},
		vector: vectorCopy,
		column: column,
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

// Retrieve statistics about an index
func (t *Table) IndexStats(_ context.Context, indexName string) (*contracts.IndexStatistics, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	cIndexName := C.CString(indexName)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cIndexName))

	var indexStatsJSON *C.char
	result := C.simple_lancedb_table_index_stats(t.handle, cIndexName, &indexStatsJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to get indexes: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to get indexes: unknown error")
	}

	if indexStatsJSON == nil {
		return nil, nil
	}

	jsonStr := C.GoString(indexStatsJSON)
	C.simple_lancedb_free_string(indexStatsJSON)

	// Parse JSON response
	var indexStats contracts.IndexStatistics
	if err := json.Unmarshal([]byte(jsonStr), &indexStats); err != nil {
		return nil, fmt.Errorf("failed to parse index stats JSON: %w", err)
	}

	return &indexStats, nil
}

// WaitForIndex blocks until every index in `names` reports zero unindexed
// rows or the deadline elapses. An empty `names` slice defers to the
// backend's "all indices on this table" behaviour.
//
// Deadline composition (most-restrictive wins, then forwarded to the
// Rust side as a single millisecond budget):
//   - `timeout > 0`           — explicit upper bound from the caller.
//   - `ctx` carries a deadline — narrows the budget to whichever expires
//     first. This makes `context.WithTimeout` actually bound the FFI call,
//     which it didn't in the prior revision.
//   - `timeout < 0`           — treated as "no wait" (1ms floor).
//   - `timeout == 0` and ctx has no deadline — wait forever (Rust's
//     `Duration::MAX`); abort only via process exit.
//
// Sub-millisecond positive durations are rounded up to 1ms instead of
// truncating to 0, which the Rust side would otherwise interpret as
// "wait forever".
//
// Implementation note: lancedb-rust's wait_for_index isn't cancellation
// aware once it starts, so a bare `ctx.Cancel()` (no deadline) will not
// interrupt an in-flight call. Callers needing tight cancellation should
// pass a deadline-bearing ctx or a positive `timeout`.
//
//nolint:gocritic
func (t *Table) WaitForIndex(ctx context.Context, names []string, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	// Allocate C strings for each name; keep them alive until after the
	// FFI call returns so the pointers we hand in stay valid.
	cNames := make([]*C.char, len(names))
	for i, n := range names {
		cNames[i] = C.CString(n)
	}
	defer func() {
		for _, p := range cNames {
			// #nosec G103 - Required for freeing C allocated string memory
			C.free(unsafe.Pointer(p))
		}
	}()

	var namesPtr **C.char
	if len(cNames) > 0 {
		namesPtr = (**C.char)(unsafe.Pointer(&cNames[0]))
	}

	timeoutMs := ComputeWaitTimeoutMs(ctx, timeout)

	result := C.simple_lancedb_table_wait_for_index(
		t.handle,
		namesPtr,
		C.size_t(len(cNames)),
		C.uint64_t(timeoutMs),
	)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("wait_for_index failed: %s", errorMsg)
		}
		return fmt.Errorf("wait_for_index failed: unknown error")
	}
	return nil
}

// ComputeWaitTimeoutMs folds the caller's `timeout` and any deadline
// carried by ctx into a single millisecond budget for the Rust side.
// Returns 0 only when both the caller asks for no upper bound and ctx
// carries no deadline — that's the only path the Rust side maps to
// Duration::MAX. Sub-millisecond positive budgets are rounded up to 1
// to avoid the truncation bug that would otherwise surface them as 0.
//
// Exported only so the table-tests package can pin these invariants
// without spinning up a LanceDB connection per case; not part of the
// public surface (the contracts package does not re-export it).
func ComputeWaitTimeoutMs(ctx context.Context, timeout time.Duration) uint64 {
	// Negative caller timeout: emulate "no wait" with a 1ms floor so the
	// FFI call returns promptly instead of hanging on Duration::MAX.
	if timeout < 0 {
		return 1
	}

	effective := timeout
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining <= 0 {
			return 1 // deadline already passed; let the FFI return ASAP
		}
		if effective == 0 || remaining < effective {
			effective = remaining
		}
	}

	if effective == 0 {
		return 0 // caller wants unbounded and ctx has no deadline
	}

	ms := effective.Milliseconds()
	if ms < 1 {
		ms = 1 // floor sub-ms positive durations so they aren't truncated to 0
	}
	return uint64(ms)
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

// SelectIPC executes a query and returns raw Arrow IPC bytes.
// The caller must deserialize the IPC data into Arrow Records.
//
//nolint:gocritic
func (t *Table) SelectIPC(_ context.Context, config contracts.QueryConfig) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query config to JSON: %w", err)
	}

	cConfigJSON := C.CString(string(configJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cConfigJSON))

	var resultIPCData *C.uchar
	var resultIPCLen C.size_t
	result := C.simple_lancedb_table_select_query_ipc(t.handle, cConfigJSON, &resultIPCData, &resultIPCLen)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to execute IPC query: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to execute IPC query: unknown error")
	}

	if resultIPCData == nil || resultIPCLen == 0 {
		return nil, nil // Empty result set
	}

	// Guard against integer truncation for payloads > 2GB
	if resultIPCLen > C.size_t(math.MaxInt32) {
		C.simple_lancedb_free_ipc_data(resultIPCData)
		return nil, fmt.Errorf("IPC result too large (%d bytes) to copy", resultIPCLen)
	}

	// Copy IPC data to Go-owned memory and free C allocation
	// #nosec G103 - Safe conversion of C memory to Go bytes for Arrow IPC data
	ipcBytes := C.GoBytes(unsafe.Pointer(resultIPCData), C.int(resultIPCLen))
	C.simple_lancedb_free_ipc_data(resultIPCData)

	return ipcBytes, nil
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

// Optimize the on-disk data and indices for better performance
func (t *Table) Optimize(_ context.Context) (*contracts.OptimizeStats, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	var optimizeStatsJSON *C.char
	result := C.simple_lancedb_table_optimize(t.handle, &optimizeStatsJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to optimize table: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to optimize table: unknown error")
	}

	if optimizeStatsJSON == nil {
		return nil, fmt.Errorf("optimize stats is nil")
	}

	jsonStr := C.GoString(optimizeStatsJSON)
	C.simple_lancedb_free_string(optimizeStatsJSON)

	// Parse JSON response
	var optimizeStats contracts.OptimizeStats
	if err := json.Unmarshal([]byte(jsonStr), &optimizeStats); err != nil {
		return nil, fmt.Errorf("failed to parse optimize stats JSON: %w", err)
	}

	return &optimizeStats, nil
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

// CreateIndexWithParams creates an index using the v2 FFI entry point,
// which accepts a JSON config carrying per-type tuning parameters plus
// name/replace/wait_timeout. `opts` may be nil for defaults.
//
//nolint:gocritic
func (t *Table) CreateIndexWithParams(
	_ context.Context,
	columns []string,
	indexType contracts.IndexType,
	params contracts.IndexParams,
	opts *contracts.CreateIndexOptions,
) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	columnsJSON, err := json.Marshal(columns)
	if err != nil {
		return fmt.Errorf("failed to marshal columns: %w", err)
	}

	// Embed IndexParams' fields into an envelope carrying type +
	// distance_type, which aren't part of the params struct's JSON
	// surface. Single Marshal; no intermediate map.
	envelope := struct {
		Type         string `json:"type"`
		DistanceType string `json:"distance_type,omitempty"`
		contracts.IndexParams
	}{
		Type:        t.indexTypeToString(indexType),
		IndexParams: params,
	}
	if params.DistanceType != contracts.DistanceTypeUnspecified {
		dt, err := distanceTypeToString(params.DistanceType)
		if err != nil {
			return fmt.Errorf("invalid IndexParams.DistanceType: %w", err)
		}
		envelope.DistanceType = dt
	}
	cfgJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal index config: %w", err)
	}

	var (
		name          string
		replace       bool
		waitTimeoutMs uint64
	)
	if opts != nil {
		name = opts.Name
		replace = opts.Replace
		if opts.WaitTimeout > 0 {
			ms := opts.WaitTimeout.Milliseconds()
			if ms > 0 {
				waitTimeoutMs = uint64(ms)
			}
		}
	}

	cColumnsJSON := C.CString(string(columnsJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cColumnsJSON))

	cConfigJSON := C.CString(string(cfgJSON))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cConfigJSON))

	var cName *C.char
	if name != "" {
		cName = C.CString(name)
		// #nosec G103 - Required for freeing C allocated string memory
		defer C.free(unsafe.Pointer(cName))
	}

	result := C.simple_lancedb_table_create_index_v2(
		t.handle,
		cColumnsJSON,
		cConfigJSON,
		cName,
		C.bool(replace),
		C.uint64_t(waitTimeoutMs),
	)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("create_index_v2 failed: %s", errorMsg)
		}
		return fmt.Errorf("create_index_v2 failed: unknown error")
	}
	return nil
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
