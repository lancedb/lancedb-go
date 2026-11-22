// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

// Table is a live lancedb-c table handle.
type Table struct {
	name       string
	connection *Connection
	handle     uintptr
	mu         sync.RWMutex
	closed     bool
}

var (
	_ contracts.ITable                = (*Table)(nil)
	_ contracts.ITableQueryV2         = (*Table)(nil)
	_ contracts.ITableVectorQuery     = (*Table)(nil)
	_ contracts.ITableMergeInsert     = (*Table)(nil)
	_ contracts.ITableIndexManagement = (*Table)(nil)
	_ contracts.ITableOptimize        = (*Table)(nil)
	_ contracts.ITableUpdateExpr      = (*Table)(nil)
	_ contracts.ITableTimeTravel      = (*Table)(nil)
	_ contracts.ITableSchemaEvolve    = (*Table)(nil)
	_ contracts.ITablePrewarmIndex    = (*Table)(nil)
)

func (t *Table) Name() string { return t.name }

func (t *Table) IsOpen() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed && t.handle != 0 && t.connection != nil && !t.connection.IsClosed()
}

func (t *Table) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.handle == 0 {
		return nil
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	api.TableFree(t.handle)
	t.handle = 0
	t.closed = true
	runtime.SetFinalizer(t, nil)
	return nil
}

func (t *Table) finalize() {
	_ = t.Close()
}

func (t *Table) Schema(_ context.Context) (*arrow.Schema, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	ipcBytes, err := tableSchemaIPC(api, t.handle)
	if err != nil {
		return nil, err
	}
	records, schema, err := ipcBytesToRecords(ipcBytes)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		record.Release()
	}
	if schema == nil {
		return nil, fmt.Errorf("lancedb-c returned an empty table schema")
	}
	return schema, nil
}

func (t *Table) Add(ctx context.Context, record arrow.Record, options *contracts.AddDataOptions) error {
	if record == nil {
		t.mu.RLock()
		closed := t.closed || t.handle == 0
		t.mu.RUnlock()
		if closed {
			return fmt.Errorf("table is closed")
		}
		return nil
	}
	return t.AddRecords(ctx, []arrow.Record{record}, options)
}

func (t *Table) AddRecords(_ context.Context, records []arrow.Record, _ *contracts.AddDataOptions) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	if len(records) == 0 {
		return nil
	}
	ipcBytes, err := recordsToIPCBytes(records)
	if err != nil {
		return err
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	reader, _, err := newIPCReader(api, ipcBytes, false)
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableAdd(t.handle, reader, &errorMessage)
	return check(api, code, errorMessage, "failed to add records")
}

func (t *Table) Count(_ context.Context) (int64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	var count uintptr
	var errorMessage *byte
	code := api.TableCountRowsWithStatus(t.handle, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to count rows"); err != nil {
		return 0, err
	}
	if uint64(count) > math.MaxInt64 {
		return 0, fmt.Errorf("row count %d overflows int64", count)
	}
	return int64(count), nil
}

func (t *Table) Version(_ context.Context) (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	var version uint64
	var errorMessage *byte
	code := api.TableVersionWithStatus(t.handle, &version, &errorMessage)
	if err := check(api, code, errorMessage, "failed to get table version"); err != nil {
		return 0, err
	}
	if version > math.MaxInt {
		return 0, fmt.Errorf("table version %d overflows int", version)
	}
	return int(version), nil
}

func (t *Table) Update(_ context.Context, filter string, updates map[string]interface{}) error {
	if strings.TrimSpace(filter) == "" {
		return fmt.Errorf("failed to update rows: predicate must not be empty")
	}
	if len(updates) == 0 {
		return fmt.Errorf("updates map cannot be empty")
	}
	columns := make([]string, 0, len(updates))
	for column := range updates {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	expressions := make([]string, len(columns))
	for i, column := range columns {
		expression, err := legacyJSONScalar(updates[column])
		if err != nil {
			return fmt.Errorf("update column %s: %w", column, err)
		}
		expressions[i] = expression
	}
	_, err := t.updateExpressions(filter, columns, expressions)
	return err
}

func (t *Table) UpdateExpr(_ context.Context, filter string, assignments []contracts.UpdateAssignment) (*contracts.UpdateResult, error) {
	if len(assignments) == 0 {
		return nil, fmt.Errorf("update_expr: assignments must be non-empty")
	}
	columns := make([]string, len(assignments))
	expressions := make([]string, len(assignments))
	for i, assignment := range assignments {
		if strings.TrimSpace(assignment.Column) == "" {
			return nil, fmt.Errorf("update_expr: assignments[%d].Column is empty", i)
		}
		if strings.TrimSpace(assignment.Expr) == "" {
			return nil, fmt.Errorf("update_expr: assignments[%d].Expr is empty", i)
		}
		columns[i] = assignment.Column
		expressions[i] = assignment.Expr
	}
	return t.updateExpressions(filter, columns, expressions)
}

func (t *Table) updateExpressions(filter string, columns, expressions []string) (*contracts.UpdateResult, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	columnPtrs, keepColumns := cStringArray(columns)
	defer keepColumns()
	expressionPtrs, keepExpressions := cStringArray(expressions)
	defer keepExpressions()

	var predicate *byte
	var predicateData []byte
	if filter != "" {
		predicate, predicateData = cStringPtr(filter)
		defer runtime.KeepAlive(predicateData)
	}
	var result native.UpdateResult
	var errorMessage *byte
	code := api.TableUpdate(t.handle, columnPtrs, expressionPtrs, uintptr(len(columns)), predicate, &result, &errorMessage)
	if err := check(api, code, errorMessage, "failed to update rows"); err != nil {
		return nil, err
	}
	return &contracts.UpdateResult{RowsUpdated: result.RowsUpdated, Version: result.Version}, nil
}

func (t *Table) Delete(_ context.Context, filter string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableDelete(t.handle, filter, &errorMessage)
	return check(api, code, errorMessage, "failed to delete rows")
}

func (t *Table) MergeInsert(on []string) contracts.IMergeInsertBuilder {
	return &MergeInsertBuilder{table: t, on: append([]string(nil), on...)}
}

// Query returns the published v0.1.2 query builder.
func (t *Table) Query() contracts.IQueryBuilder {
	state := newQueryState(t)
	state.legacy = true
	return &QueryBuilder{state: state}
}

// QueryV2 returns the additive context/single-record query builder.
func (t *Table) QueryV2() contracts.IQueryBuilderV2 {
	return &QueryBuilderV2{state: newQueryState(t)}
}

// VectorQuery returns the additive vector query builder.
func (t *Table) VectorQuery(column string, vector []float32) contracts.IVectorQueryBuilderV2 {
	vectorCopy := append([]float32(nil), vector...)
	state := newQueryState(t)
	state.vectorRequested = true
	return &VectorQueryBuilderV2{state: state, vector: vectorCopy, column: column}
}

func legacyJSONScalar(value interface{}) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("value is not JSON-serializable: %w", err)
	}
	encoded = bytes.TrimSpace(encoded)
	if len(encoded) == 0 {
		return "", fmt.Errorf("value encoded to an empty JSON scalar")
	}
	switch encoded[0] {
	case '"':
		var text string
		if err := json.Unmarshal(encoded, &text); err != nil {
			return "", fmt.Errorf("decode JSON string: %w", err)
		}
		return "'" + strings.ReplaceAll(text, "'", "''") + "'", nil
	case 'n':
		if string(encoded) == "null" {
			return "NULL", nil
		}
	case 't', 'f':
		if string(encoded) == "true" || string(encoded) == "false" {
			return string(encoded), nil
		}
	default:
		if encoded[0] == '-' || (encoded[0] >= '0' && encoded[0] <= '9') {
			// json.Marshal only produces valid JSON numbers.
			return string(encoded), nil
		}
	}
	return "", fmt.Errorf("unsupported composite JSON value %s", string(encoded))
}

// Select executes a published v0.1.2 query configuration and returns map rows.
func (t *Table) Select(ctx context.Context, config contracts.QueryConfig) ([]map[string]interface{}, error) {
	ipcBytes, err := t.SelectIPC(ctx, config)
	if err != nil {
		return nil, err
	}
	rows, err := ipcBytesToMaps(ipcBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query results: %w", err)
	}
	return rows, nil
}

// SelectIPC executes a published v0.1.2 query configuration and returns Arrow
// IPC file bytes. A schema-only result is returned as a valid IPC file; an
// entirely empty response is returned as nil.
func (t *Table) SelectIPC(ctx context.Context, config contracts.QueryConfig) ([]byte, error) {
	state := newQueryState(t)
	state.legacy = true
	state.columns = append([]string(nil), config.Columns...)
	if config.Where != "" {
		state.filters = []string{config.Where}
	}
	if config.VectorSearch != nil {
		state.vectorRequested = true
		state.vector = append([]float32(nil), config.VectorSearch.Vector...)
		state.vectorColumn = config.VectorSearch.Column
		if config.Limit != nil && *config.Limit >= 0 {
			state.limit = *config.Limit
			state.limitSet = true
			state.zeroLimit = state.limit == 0
		} else {
			state.limit = config.VectorSearch.K
			state.limitSet = true
			state.zeroLimit = state.limit == 0
		}
		// v0.1.2 ignores Offset for vector queries.
	} else if config.FTSSearch != nil {
		return nil, fmt.Errorf("failed to execute select query: Invalid input, Full-text search is not currently supported")
	} else {
		if config.Limit != nil && *config.Limit >= 0 {
			state.limit = *config.Limit
			state.limitSet = true
			state.zeroLimit = state.limit == 0
		}
		if config.Offset != nil && *config.Offset >= 0 {
			state.offset = *config.Offset
		}
	}
	return t.executeQueryIPC(ctx, state)
}

// SelectV2 executes an additive query configuration and returns map rows.
func (t *Table) SelectV2(ctx context.Context, config contracts.QueryConfigV2) ([]map[string]interface{}, error) {
	ipcBytes, err := t.SelectIPCV2(ctx, config)
	if err != nil {
		return nil, err
	}
	return ipcBytesToMaps(ipcBytes)
}

// SelectIPCV2 executes an additive query configuration and returns Arrow IPC
// file bytes.
func (t *Table) SelectIPCV2(ctx context.Context, config contracts.QueryConfigV2) ([]byte, error) {
	state := newQueryState(t)
	state.columns = append([]string(nil), config.Columns...)
	if config.Where != "" {
		state.filters = []string{config.Where}
	}
	if config.Limit != nil {
		state.limit = *config.Limit
		state.limitSet = true
	}
	if config.Offset != nil {
		state.offset = *config.Offset
	}
	if config.VectorSearch != nil {
		state.vectorRequested = true
		state.vector = append([]float32(nil), config.VectorSearch.Vector...)
		state.vectorColumn = config.VectorSearch.Column
		if !state.limitSet {
			state.limit = config.VectorSearch.K
			state.limitSet = true
		}
		state.distanceType = config.VectorSearch.DistanceType
		state.nprobes = config.VectorSearch.Nprobes
		state.refineFactor = config.VectorSearch.RefineFactor
		state.ef = config.VectorSearch.Ef
		state.bypassVectorIndex = config.VectorSearch.BypassVectorIndex
		state.fullTextQuery = config.VectorSearch.FullTextQuery
		state.fullTextColumn = config.VectorSearch.FullTextColumn
	} else if config.FTSSearch != nil {
		if strings.TrimSpace(config.FTSSearch.Query) == "" {
			return nil, fmt.Errorf("full-text search query must not be empty")
		}
		state.fullTextQuery = config.FTSSearch.Query
		state.fullTextColumn = config.FTSSearch.Column
	}
	if config.Postfilter && config.VectorSearch == nil {
		return nil, fmt.Errorf("postfilter requires a vector query")
	}
	if config.Reranker != nil && config.Reranker.Kind != contracts.RerankerNone && config.VectorSearch == nil {
		return nil, fmt.Errorf("reranker requires a vector query")
	}
	state.withRowID = config.WithRowID
	state.fastSearch = config.FastSearch
	state.postfilter = config.Postfilter
	state.reranker = config.Reranker
	return t.executeQueryIPC(ctx, state)
}

func (t *Table) SelectWithColumns(ctx context.Context, columns []string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{Columns: columns})
}

func (t *Table) SelectWithFilter(ctx context.Context, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{Where: filter})
}

func (t *Table) VectorSearch(ctx context.Context, column string, vector []float32, k int) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{VectorSearch: &contracts.VectorSearch{Column: column, Vector: vector, K: k}})
}

func (t *Table) VectorSearchWithFilter(ctx context.Context, column string, vector []float32, k int, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{VectorSearch: &contracts.VectorSearch{Column: column, Vector: vector, K: k}, Where: filter})
}

func (t *Table) FullTextSearch(ctx context.Context, column string, query string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{FTSSearch: &contracts.FTSSearch{Column: column, Query: query}})
}

func (t *Table) FullTextSearchWithFilter(ctx context.Context, column string, query string, filter string) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{FTSSearch: &contracts.FTSSearch{Column: column, Query: query}, Where: filter})
}

func (t *Table) SelectWithLimit(ctx context.Context, limit int, offset int) ([]map[string]interface{}, error) {
	return t.Select(ctx, contracts.QueryConfig{Limit: &limit, Offset: &offset})
}
