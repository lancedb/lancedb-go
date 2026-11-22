// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

import (
	"context"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
)

// IQueryBuilderV2 is the additive query builder used by ITableQueryV2. It is
// separate from the published IQueryBuilder because Go cannot overload Execute
// by signature; v0.1.2 callers keep Execute() ([]arrow.Record, error).
type IQueryBuilderV2 interface {
	Filter(condition string) IQueryBuilderV2
	Limit(limit int) IQueryBuilderV2
	Columns(columns []string) IQueryBuilderV2
	Offset(offset int) IQueryBuilderV2
	WithRowID() IQueryBuilderV2
	FastSearch() IQueryBuilderV2
	Execute(ctx context.Context) (arrow.Record, error)
	ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IQueryBuilderV2
}

// IVectorQueryBuilderV2 is the additive vector query builder.
type IVectorQueryBuilderV2 interface {
	Filter(condition string) IVectorQueryBuilderV2
	Limit(limit int) IVectorQueryBuilderV2
	Columns(columns []string) IVectorQueryBuilderV2
	DistanceType(dt DistanceType) IVectorQueryBuilderV2
	Nprobes(n int) IVectorQueryBuilderV2
	RefineFactor(n uint32) IVectorQueryBuilderV2
	Ef(n int) IVectorQueryBuilderV2
	BypassVectorIndex() IVectorQueryBuilderV2
	WithRowID() IVectorQueryBuilderV2
	FastSearch() IVectorQueryBuilderV2
	Postfilter() IVectorQueryBuilderV2
	Rerank(cfg RerankerConfig) IVectorQueryBuilderV2
	WithFullText(query, column string) IVectorQueryBuilderV2
	Execute(ctx context.Context) (arrow.Record, error)
	ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IVectorQueryBuilderV2
}

// ITableQueryV2 exposes the context/single-record query builder and the
// additive SelectV2 entry points. The published ITable.Query() keeps its
// v0.1.2 return type.
type ITableQueryV2 interface {
	QueryV2() IQueryBuilderV2
	SelectV2(ctx context.Context, config QueryConfigV2) ([]map[string]interface{}, error)
	SelectIPCV2(ctx context.Context, config QueryConfigV2) ([]byte, error)
}

// ITableVectorQuery exposes the vector builder introduced after v0.1.2.
type ITableVectorQuery interface {
	VectorQuery(column string, vector []float32) IVectorQueryBuilderV2
}

// ITableMergeInsert exposes merge_insert.
type ITableMergeInsert interface {
	MergeInsert(on []string) IMergeInsertBuilder
}

// ITableIndexManagement exposes index statistics, waits, drops and full
// parameterized creation.
type ITableIndexManagement interface {
	CreateIndexWithParams(ctx context.Context, columns []string, indexType IndexType, params IndexParams, opts *CreateIndexOptions) error
	DropIndex(ctx context.Context, name string) error
	IndexStats(ctx context.Context, indexName string) (*IndexStatistics, error)
	WaitForIndex(ctx context.Context, names []string, timeout time.Duration) error
}

// ITableOptimize exposes optimize actions and their statistics.
type ITableOptimize interface {
	Optimize(ctx context.Context) (*OptimizeStats, error)
	OptimizeWithAction(ctx context.Context, action OptimizeAction) (*OptimizeStats, error)
}

// ITableUpdateExpr is an optional capability extension layered on top of
// ITable. Callers detect it with a type assertion.
type ITableUpdateExpr interface {
	UpdateExpr(ctx context.Context, filter string, assignments []UpdateAssignment) (*UpdateResult, error)
}

// ITableTimeTravel exposes version history and tag CRUD.
type ITableTimeTravel interface {
	ListVersions(ctx context.Context) ([]VersionInfo, error)
	Checkout(ctx context.Context, version uint64) error
	CheckoutTag(ctx context.Context, tag string) error
	CheckoutLatest(ctx context.Context) error
	Restore(ctx context.Context) error
	TagList(ctx context.Context) (map[string]TagInfo, error)
	TagGetVersion(ctx context.Context, tag string) (uint64, error)
	TagCreate(ctx context.Context, tag string, version uint64) error
	TagDelete(ctx context.Context, tag string) error
	TagUpdate(ctx context.Context, tag string, version uint64) error
}

// ITableSchemaEvolve exposes SQL-derived columns, renames/nullability and
// column drops.
type ITableSchemaEvolve interface {
	AddColumns(ctx context.Context, transforms []NewColumnTransform) (uint64, error)
	AlterColumns(ctx context.Context, alterations []ColumnAlteration) (uint64, error)
	DropColumns(ctx context.Context, names []string) (uint64, error)
}

// ITablePrewarmIndex exposes index prewarming.
type ITablePrewarmIndex interface {
	PrewarmIndex(ctx context.Context, name string) error
}
