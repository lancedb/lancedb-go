// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

type queryState struct {
	table *Table

	legacy          bool
	filters         []string
	limit           int
	limitSet        bool
	zeroLimit       bool
	offset          int
	columns         []string
	vectorRequested bool

	withRowID  bool
	fastSearch bool
	postfilter bool
	reranker   *contracts.RerankerConfig

	vector            []float32
	vectorColumn      string
	distanceType      *contracts.DistanceType
	nprobes           *int
	refineFactor      *uint32
	ef                *int
	bypassVectorIndex bool
	fullTextQuery     string
	fullTextColumn    string
}

func newQueryState(table *Table) *queryState {
	return &queryState{table: table}
}

// QueryBuilder implements the published v0.1.2 IQueryBuilder. Execute keeps
// the slice-of-records shape and all-batches behavior of v0.1.2.
type QueryBuilder struct {
	state *queryState
}

var _ contracts.IQueryBuilder = (*QueryBuilder)(nil)

func (q *QueryBuilder) Filter(condition string) contracts.IQueryBuilder {
	q.state.filters = append(q.state.filters, condition)
	return q
}

func (q *QueryBuilder) Limit(limit int) contracts.IQueryBuilder {
	q.state.limit = limit
	q.state.limitSet = true
	return q
}

func (q *QueryBuilder) Columns(columns []string) contracts.IQueryBuilder {
	q.state.columns = append([]string(nil), columns...)
	return q
}

func (q *QueryBuilder) Offset(offset int) contracts.IQueryBuilder {
	q.state.offset = offset
	return q
}

func (q *QueryBuilder) ApplyOptions(options *contracts.QueryOptions) contracts.IQueryBuilder {
	if options != nil && options.MaxResults > 0 {
		q.Limit(options.MaxResults)
	}
	return q
}

func (q *QueryBuilder) Execute() ([]arrow.Record, error) {
	if q.state.table.connection != nil && q.state.table.connection.IsClosed() {
		return nil, fmt.Errorf("table is closed")
	}
	return q.state.table.executeQueryRecords(context.Background(), q.state)
}

func (q *QueryBuilder) ExecuteAsync() (<-chan []arrow.Record, <-chan error) {
	results := make(chan []arrow.Record, 1)
	errs := make(chan error, 1)
	go func() {
		records, err := q.Execute()
		if err != nil {
			errs <- err
		} else {
			results <- records
		}
		close(results)
		close(errs)
	}()
	return results, errs
}

// QueryBuilderV2 implements the additive context/single-record builder.
type QueryBuilderV2 struct {
	state *queryState
}

var _ contracts.IQueryBuilderV2 = (*QueryBuilderV2)(nil)

func (q *QueryBuilderV2) Filter(condition string) contracts.IQueryBuilderV2 {
	q.state.filters = append(q.state.filters, condition)
	return q
}

func (q *QueryBuilderV2) Limit(limit int) contracts.IQueryBuilderV2 {
	q.state.limit = limit
	q.state.limitSet = true
	return q
}

func (q *QueryBuilderV2) Columns(columns []string) contracts.IQueryBuilderV2 {
	q.state.columns = append([]string(nil), columns...)
	return q
}

func (q *QueryBuilderV2) Offset(offset int) contracts.IQueryBuilderV2 {
	q.state.offset = offset
	return q
}

func (q *QueryBuilderV2) WithRowID() contracts.IQueryBuilderV2 {
	q.state.withRowID = true
	return q
}

func (q *QueryBuilderV2) FastSearch() contracts.IQueryBuilderV2 {
	q.state.fastSearch = true
	return q
}

func (q *QueryBuilderV2) ApplyOptions(options *contracts.QueryOptions) contracts.IQueryBuilderV2 {
	if options != nil && options.MaxResults > 0 {
		q.Limit(options.MaxResults)
	}
	return q
}

func (q *QueryBuilderV2) Execute(ctx context.Context) (arrow.Record, error) {
	return q.state.table.executeQueryRecord(ctx, q.state)
}

func (q *QueryBuilderV2) ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error) {
	results := make(chan arrow.Record, 1)
	errs := make(chan error, 1)
	go func() {
		record, err := q.Execute(ctx)
		if err != nil {
			errs <- err
		} else {
			results <- record
		}
		close(results)
		close(errs)
	}()
	return results, errs
}

// VectorQueryBuilderV2 implements the additive vector query builder.
type VectorQueryBuilderV2 struct {
	state  *queryState
	vector []float32
	column string
}

var _ contracts.IVectorQueryBuilderV2 = (*VectorQueryBuilderV2)(nil)

func (q *VectorQueryBuilderV2) Filter(condition string) contracts.IVectorQueryBuilderV2 {
	q.state.filters = append(q.state.filters, condition)
	return q
}

func (q *VectorQueryBuilderV2) Limit(limit int) contracts.IVectorQueryBuilderV2 {
	q.state.limit = limit
	q.state.limitSet = true
	return q
}

func (q *VectorQueryBuilderV2) Columns(columns []string) contracts.IVectorQueryBuilderV2 {
	q.state.columns = append([]string(nil), columns...)
	return q
}

func (q *VectorQueryBuilderV2) DistanceType(dt contracts.DistanceType) contracts.IVectorQueryBuilderV2 {
	copy := dt
	q.state.distanceType = &copy
	return q
}

func (q *VectorQueryBuilderV2) Nprobes(n int) contracts.IVectorQueryBuilderV2 {
	copy := n
	q.state.nprobes = &copy
	return q
}

func (q *VectorQueryBuilderV2) RefineFactor(n uint32) contracts.IVectorQueryBuilderV2 {
	copy := n
	q.state.refineFactor = &copy
	return q
}

func (q *VectorQueryBuilderV2) Ef(n int) contracts.IVectorQueryBuilderV2 {
	copy := n
	q.state.ef = &copy
	return q
}

func (q *VectorQueryBuilderV2) BypassVectorIndex() contracts.IVectorQueryBuilderV2 {
	q.state.bypassVectorIndex = true
	return q
}

func (q *VectorQueryBuilderV2) WithRowID() contracts.IVectorQueryBuilderV2 {
	q.state.withRowID = true
	return q
}

func (q *VectorQueryBuilderV2) FastSearch() contracts.IVectorQueryBuilderV2 {
	q.state.fastSearch = true
	return q
}

func (q *VectorQueryBuilderV2) Postfilter() contracts.IVectorQueryBuilderV2 {
	q.state.postfilter = true
	return q
}

func (q *VectorQueryBuilderV2) Rerank(config contracts.RerankerConfig) contracts.IVectorQueryBuilderV2 {
	copy := config
	q.state.reranker = &copy
	return q
}

func (q *VectorQueryBuilderV2) WithFullText(query, column string) contracts.IVectorQueryBuilderV2 {
	q.state.fullTextQuery = query
	q.state.fullTextColumn = column
	return q
}

func (q *VectorQueryBuilderV2) ApplyOptions(options *contracts.QueryOptions) contracts.IVectorQueryBuilderV2 {
	if options != nil {
		if options.MaxResults > 0 {
			q.Limit(options.MaxResults)
		}
		if options.BypassVectorIndex {
			q.BypassVectorIndex()
		}
	}
	return q
}

func (q *VectorQueryBuilderV2) Execute(ctx context.Context) (arrow.Record, error) {
	q.state.vector = append([]float32(nil), q.vector...)
	q.state.vectorColumn = q.column
	q.state.vectorRequested = true
	return q.state.table.executeQueryRecord(ctx, q.state)
}

func (q *VectorQueryBuilderV2) ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error) {
	results := make(chan arrow.Record, 1)
	errs := make(chan error, 1)
	go func() {
		record, err := q.Execute(ctx)
		if err != nil {
			errs <- err
		} else {
			results <- record
		}
		close(results)
		close(errs)
	}()
	return results, errs
}

func (t *Table) executeQueryRecords(ctx context.Context, state *queryState) ([]arrow.Record, error) {
	ipcBytes, err := t.executeQueryIPC(ctx, state)
	if err != nil {
		return nil, err
	}
	records, _, err := ipcBytesToRecords(ipcBytes)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return []arrow.Record{}, nil
	}
	return records, nil
}

func (t *Table) executeQueryRecord(ctx context.Context, state *queryState) (arrow.Record, error) {
	ipcBytes, err := t.executeQueryIPC(ctx, state)
	if err != nil {
		return nil, err
	}
	return ipcBytesToRecord(ipcBytes)
}

func (t *Table) executeQueryIPC(ctx context.Context, state *queryState) ([]byte, error) {
	if !state.legacy {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	vectorQuery := state.vectorRequested
	if !vectorQuery && state.limitSet && state.limit < 0 {
		return nil, fmt.Errorf("query limit must not be negative")
	}
	var query uintptr
	if vectorQuery {
		if len(state.vector) == 0 {
			if state.legacy {
				return nil, fmt.Errorf("failed to execute select query: Query vector must have non-zero length")
			}
			return nil, fmt.Errorf("vector search requires a non-empty query vector")
		}
		if state.vectorColumn == "" {
			return nil, fmt.Errorf("vector search requires a non-empty column name")
		}
		if !state.limitSet {
			return nil, fmt.Errorf("vector search requires a positive K value; call Limit or set QueryOptions.MaxResults")
		}
		if state.limit < 0 || (!state.legacy && state.limit == 0) {
			return nil, fmt.Errorf("K must be a positive integer")
		}
		query = api.VectorQueryNew(t.handle, &state.vector[0], uintptr(len(state.vector)))
		runtime.KeepAlive(state.vector)
	} else {
		query = api.QueryNew(t.handle)
	}
	if query == 0 {
		return nil, fmt.Errorf("failed to create native query")
	}
	if vectorQuery {
		var columnError *byte
		if err := check(api, api.VectorQueryColumn(query, state.vectorColumn, &columnError), columnError, "set vector column"); err != nil {
			api.VectorQueryFree(query)
			return nil, err
		}
	}
	executed := false
	defer func() {
		if !executed {
			if vectorQuery {
				api.VectorQueryFree(query)
			} else {
				api.QueryFree(query)
			}
		}
	}()

	apply := func(code int32, errorMessage *byte, operation string) error {
		return check(api, code, errorMessage, operation)
	}
	var errorMessage *byte

	if state.offset < 0 {
		if !state.legacy {
			return nil, fmt.Errorf("query offset must not be negative")
		}
	} else if state.offset > 0 && !(state.legacy && vectorQuery) {
		if vectorQuery {
			if err := apply(api.VectorQueryOffset(query, uintptr(state.offset), &errorMessage), errorMessage, "set query offset"); err != nil {
				return nil, err
			}
		} else {
			if err := apply(api.QueryOffset(query, uintptr(state.offset), &errorMessage), errorMessage, "set query offset"); err != nil {
				return nil, err
			}
		}
	}
	if state.limitSet && (state.limit > 0 || state.zeroLimit) {
		if vectorQuery {
			if err := apply(api.VectorQueryLimit(query, uintptr(state.limit), &errorMessage), errorMessage, "set query limit"); err != nil {
				return nil, err
			}
		} else {
			if err := apply(api.QueryLimit(query, uintptr(state.limit), &errorMessage), errorMessage, "set query limit"); err != nil {
				return nil, err
			}
		}
	}
	if len(state.columns) > 0 {
		columnPtrs, keep := cStringArray(state.columns)
		if vectorQuery {
			err = apply(api.VectorQuerySelect(query, columnPtrs, uintptr(len(state.columns)), &errorMessage), errorMessage, "set query columns")
		} else {
			err = apply(api.QuerySelect(query, columnPtrs, uintptr(len(state.columns)), &errorMessage), errorMessage, "set query columns")
		}
		keep()
		if err != nil {
			return nil, err
		}
	}
	if state.withRowID {
		if vectorQuery {
			if err := apply(api.VectorQueryWithRowID(query, &errorMessage), errorMessage, "add row id"); err != nil {
				return nil, err
			}
		} else {
			if err := apply(api.QueryWithRowID(query, &errorMessage), errorMessage, "add row id"); err != nil {
				return nil, err
			}
		}
	}
	if state.fastSearch {
		if vectorQuery {
			if err := apply(api.VectorQueryFastSearch(query, &errorMessage), errorMessage, "enable fast search"); err != nil {
				return nil, err
			}
		} else {
			if err := apply(api.QueryFastSearch(query, &errorMessage), errorMessage, "enable fast search"); err != nil {
				return nil, err
			}
		}
	}
	if state.postfilter && vectorQuery {
		if err := apply(api.VectorQueryPostfilter(query, &errorMessage), errorMessage, "enable postfilter"); err != nil {
			return nil, err
		}
	}
	if len(state.filters) > 0 {
		filter := strings.Join(state.filters, " AND ")
		if filter != "" {
			if vectorQuery {
				if err := apply(api.VectorQueryWhereFilter(query, filter, &errorMessage), errorMessage, "set query filter"); err != nil {
					return nil, err
				}
			} else {
				if err := apply(api.QueryWhereFilter(query, filter, &errorMessage), errorMessage, "set query filter"); err != nil {
					return nil, err
				}
			}
		}
	}
	if state.fullTextQuery != "" {
		var columnPtr *byte
		var columnData []byte
		if state.fullTextColumn != "" {
			columnPtr, columnData = cStringPtr(state.fullTextColumn)
		}
		if vectorQuery {
			err = apply(api.VectorQueryFullTextSearch(query, state.fullTextQuery, columnPtr, &errorMessage), errorMessage, "set full-text search")
		} else {
			err = apply(api.QueryFullTextSearch(query, state.fullTextQuery, columnPtr, &errorMessage), errorMessage, "set full-text search")
		}
		if columnData != nil {
			runtime.KeepAlive(columnData)
		}
		if err != nil {
			return nil, err
		}
	}
	if vectorQuery {
		if state.distanceType != nil {
			distance, err := distanceTypeToNative(*state.distanceType)
			if err != nil {
				return nil, err
			}
			if err := apply(api.VectorQueryDistanceType(query, distance, &errorMessage), errorMessage, "set distance type"); err != nil {
				return nil, err
			}
		}
		if state.nprobes != nil && *state.nprobes > 0 {
			if err := apply(api.VectorQueryNprobes(query, uintptr(*state.nprobes), &errorMessage), errorMessage, "set nprobes"); err != nil {
				return nil, err
			}
		}
		if state.refineFactor != nil && *state.refineFactor > 0 {
			if err := apply(api.VectorQueryRefineFactor(query, *state.refineFactor, &errorMessage), errorMessage, "set refine factor"); err != nil {
				return nil, err
			}
		}
		if state.ef != nil && *state.ef > 0 {
			if err := apply(api.VectorQueryEf(query, uintptr(*state.ef), &errorMessage), errorMessage, "set ef"); err != nil {
				return nil, err
			}
		}
		if state.bypassVectorIndex {
			if err := apply(api.VectorQueryBypassVectorIndex(query, &errorMessage), errorMessage, "bypass vector index"); err != nil {
				return nil, err
			}
		}
		if state.reranker != nil && state.reranker.Kind == contracts.RerankerRRF {
			k := state.reranker.RRFK
			if k <= 0 {
				k = 60
			}
			if err := apply(api.VectorQueryRerankRRF(query, k, &errorMessage), errorMessage, "set RRF reranker"); err != nil {
				return nil, err
			}
			switch state.reranker.Norm {
			case contracts.NormalizeScore:
				if err := apply(api.VectorQueryNorm(query, native.NormalizeScore, &errorMessage), errorMessage, "set normalization"); err != nil {
					return nil, err
				}
			case contracts.NormalizeRank:
				if err := apply(api.VectorQueryNorm(query, native.NormalizeRank, &errorMessage), errorMessage, "set normalization"); err != nil {
					return nil, err
				}
			}
		}
	}

	var result uintptr
	if vectorQuery {
		err = apply(api.VectorQueryExecuteWithStatus(query, &result, &errorMessage), errorMessage, "execute vector query")
	} else {
		err = apply(api.QueryExecuteWithStatus(query, &result, &errorMessage), errorMessage, "execute query")
	}
	executed = true
	if err != nil {
		return nil, err
	}
	if result == 0 {
		return nil, fmt.Errorf("native query returned a null result")
	}
	return queryResultIPC(api, result)
}

func distanceTypeToNative(distance contracts.DistanceType) (int32, error) {
	switch distance {
	case contracts.DistanceTypeL2:
		return native.DistanceL2, nil
	case contracts.DistanceTypeCosine:
		return native.DistanceCosine, nil
	case contracts.DistanceTypeDot:
		return native.DistanceDot, nil
	case contracts.DistanceTypeHamming:
		return native.DistanceHamming, nil
	default:
		return 0, fmt.Errorf("unknown DistanceType: %d", distance)
	}
}
