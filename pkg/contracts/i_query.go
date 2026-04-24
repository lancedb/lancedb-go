// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

import (
	"context"

	"github.com/apache/arrow/go/v17/arrow"
)

type IQueryBuilder interface {
	Filter(condition string) IQueryBuilder
	Limit(limit int) IQueryBuilder
	Columns(columns []string) IQueryBuilder
	Offset(offset int) IQueryBuilder
	// WithRowID adds the internal _rowid column to the result.
	WithRowID() IQueryBuilder
	// FastSearch skips rows not yet covered by an index.
	FastSearch() IQueryBuilder
	// Postfilter evaluates WHERE after the candidate set is built.
	Postfilter() IQueryBuilder
	// Rerank installs a reranker on the query. Most useful in hybrid
	// search where vector and FTS scores need to be fused; on a single
	// channel the backend may noop.
	Rerank(cfg RerankerConfig) IQueryBuilder
	Execute(ctx context.Context) (arrow.Record, error)
	ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IQueryBuilder
}

type IVectorQueryBuilder interface {
	Filter(condition string) IVectorQueryBuilder
	Limit(limit int) IVectorQueryBuilder
	Columns(columns []string) IVectorQueryBuilder
	DistanceType(dt DistanceType) IVectorQueryBuilder
	// Nprobes is the IVF partition scan count. 0 leaves the backend default.
	Nprobes(n int) IVectorQueryBuilder
	// RefineFactor multiplies the first-stage IVF k. 0 leaves the default off.
	RefineFactor(n uint32) IVectorQueryBuilder
	// Ef is the HNSW candidate list size. 0 leaves the backend default.
	Ef(n int) IVectorQueryBuilder
	// BypassVectorIndex forces a flat scan instead of the trained index.
	BypassVectorIndex() IVectorQueryBuilder
	// WithRowID adds the internal _rowid column to the result.
	WithRowID() IVectorQueryBuilder
	// FastSearch skips rows not yet covered by the index.
	FastSearch() IVectorQueryBuilder
	// Postfilter evaluates WHERE after the vector candidate set is built.
	Postfilter() IVectorQueryBuilder
	// Rerank installs a reranker on the query. Most useful in hybrid
	// search where vector and FTS scores need to be fused.
	Rerank(cfg RerankerConfig) IVectorQueryBuilder
	// WithFullText turns the vector query into a hybrid vector+FTS
	// query. The matching text column must have an FTS index.
	// `column` may be empty to let lancedb pick the one indexed FTS
	// column on the table.
	WithFullText(query, column string) IVectorQueryBuilder
	Execute(ctx context.Context) (arrow.Record, error)
	ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IVectorQueryBuilder
}

// QueryOptions provides additional configuration for queries
type QueryOptions struct {
	MaxResults        int
	UseFullPrecision  bool
	BypassVectorIndex bool
}
