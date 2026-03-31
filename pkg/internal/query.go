// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/arrow/go/v17/arrow"

	lancedb "github.com/lancedb/lancedb-go/pkg/contracts"
)

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder struct {
	table   *Table
	filters []string
	limit   int
	offset  int
	columns []string
}

var _ lancedb.IQueryBuilder = (*QueryBuilder)(nil)
var _ lancedb.IVectorQueryBuilder = (*VectorQueryBuilder)(nil)

// VectorQueryBuilder extends QueryBuilder for vector similarity searches
type VectorQueryBuilder struct {
	QueryBuilder
	vector       []float32
	column       string
	limitSet     bool // tracks whether Limit() was explicitly called
	distanceType *lancedb.DistanceType
}

// Filter adds a filter condition to the query
func (q *QueryBuilder) Filter(condition string) lancedb.IQueryBuilder {
	q.filters = append(q.filters, condition)
	return q
}

// Limit sets the maximum number of results to return
func (q *QueryBuilder) Limit(limit int) lancedb.IQueryBuilder {
	q.limit = limit
	return q
}

// Columns sets the columns to return
func (q *QueryBuilder) Columns(columns []string) lancedb.IQueryBuilder {
	q.columns = columns
	return q
}

// Offset sets the number of rows to skip
func (q *QueryBuilder) Offset(offset int) lancedb.IQueryBuilder {
	q.offset = offset
	return q
}

// Execute executes the query and returns results.
// Delegates to Table.SelectIPC() which holds the mutex and checks closed state.
func (q *QueryBuilder) Execute(ctx context.Context) (arrow.Record, error) {
	config := q.buildConfig()
	ipcBytes, err := q.table.SelectIPC(ctx, config)
	if err != nil {
		return nil, err
	}
	return ipcBytesToRecord(ipcBytes)
}

// executeAsync runs fn in a goroutine and routes its result or error to
// the returned buffered channels. Exactly one channel receives a value;
// both are always closed (via defer) so callers can safely use the
// two-value receive form. Callers should check the ok flag to
// distinguish a real value (ok=true) from a closed-empty channel (ok=false)
// that may appear when the scheduler picks the other channel first.
func executeAsync(ctx context.Context, fn func(context.Context) (arrow.Record, error)) (<-chan arrow.Record, <-chan error) {
	resultChan := make(chan arrow.Record, 1)
	errorChan := make(chan error, 1)

	// Short-circuit if context is already cancelled
	if err := ctx.Err(); err != nil {
		errorChan <- err
		close(resultChan)
		close(errorChan)
		return resultChan, errorChan
	}

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		result, err := fn(ctx)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()

	return resultChan, errorChan
}

// ExecuteAsync executes the query asynchronously
func (q *QueryBuilder) ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error) {
	return executeAsync(ctx, q.Execute)
}

// ApplyOptions applies query options to the builder
func (q *QueryBuilder) ApplyOptions(options *lancedb.QueryOptions) lancedb.IQueryBuilder {
	if options != nil {
		if options.MaxResults > 0 {
			q.Limit(options.MaxResults)
		}
	}
	return q
}

// buildConfig converts the builder's accumulated state into a QueryConfig
func (q *QueryBuilder) buildConfig() lancedb.QueryConfig {
	config := lancedb.QueryConfig{}

	if len(q.filters) > 0 {
		config.Where = strings.Join(q.filters, " AND ")
	}
	if q.limit > 0 {
		limit := q.limit
		config.Limit = &limit
	}
	if q.offset > 0 {
		offset := q.offset
		config.Offset = &offset
	}
	if len(q.columns) > 0 {
		config.Columns = q.columns
	}

	return config
}

// Filter adds a filter condition to the vector query
func (vq *VectorQueryBuilder) Filter(condition string) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.Filter(condition)
	return vq
}

// Limit sets the maximum number of results to return
func (vq *VectorQueryBuilder) Limit(limit int) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.Limit(limit)
	vq.limitSet = true
	return vq
}

// Columns sets the columns to return
func (vq *VectorQueryBuilder) Columns(columns []string) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.Columns(columns)
	return vq
}

// distanceTypeToString converts a DistanceType enum to the JSON string
// expected by the Rust FFI. Caller guards against DistanceTypeUnspecified.
func distanceTypeToString(dt lancedb.DistanceType) string {
	switch dt {
	case lancedb.DistanceTypeL2:
		return "l2"
	case lancedb.DistanceTypeCosine:
		return "cosine"
	case lancedb.DistanceTypeDot:
		return "dot"
	default:
		panic(fmt.Sprintf("unhandled DistanceType: %d", dt))
	}
}

// DistanceType sets the distance metric for vector similarity search
func (vq *VectorQueryBuilder) DistanceType(dt lancedb.DistanceType) lancedb.IVectorQueryBuilder {
	vq.distanceType = &dt
	return vq
}

// Execute executes the vector search query and returns results.
// Delegates to Table.SelectIPC() which holds the mutex and checks closed state.
func (vq *VectorQueryBuilder) Execute(ctx context.Context) (arrow.Record, error) {
	if len(vq.vector) == 0 {
		return nil, fmt.Errorf("vector search requires a non-empty query vector")
	}
	if vq.column == "" {
		return nil, fmt.Errorf("vector search requires a non-empty column name")
	}

	k := vq.limit
	if !vq.limitSet {
		return nil, fmt.Errorf("vector search requires a positive K value: call .Limit(k) before .Execute()")
	}
	if k <= 0 {
		return nil, fmt.Errorf("K must be a positive integer, got %d", k)
	}

	if vq.offset != 0 {
		return nil, fmt.Errorf("VectorQueryBuilder does not support Offset(); use QueryBuilder for offset-based pagination")
	}

	config := vq.buildConfig()
	config.Limit = nil // K controls result count for vector search, not Limit
	config.VectorSearch = &lancedb.VectorSearch{
		Column: vq.column,
		Vector: vq.vector,
		K:      k,
	}
	if vq.distanceType != nil && *vq.distanceType != lancedb.DistanceTypeUnspecified {
		dt := distanceTypeToString(*vq.distanceType)
		config.VectorSearch.DistanceType = &dt
	}

	ipcBytes, err := vq.table.SelectIPC(ctx, config)
	if err != nil {
		return nil, err
	}
	return ipcBytesToRecord(ipcBytes)
}

// ExecuteAsync executes the vector query asynchronously
func (vq *VectorQueryBuilder) ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error) {
	return executeAsync(ctx, vq.Execute)
}

// ApplyOptions applies query options to the vector query builder.
// Only MaxResults is honoured; UseFullPrecision and BypassVectorIndex are
// not yet wired through the Rust FFI query path and are silently ignored.
func (vq *VectorQueryBuilder) ApplyOptions(options *lancedb.QueryOptions) lancedb.IVectorQueryBuilder {
	if options != nil && options.MaxResults > 0 {
		// Call vq.Limit() (not QueryBuilder.Limit) so limitSet is updated.
		vq.Limit(options.MaxResults)
	}
	return vq
}
