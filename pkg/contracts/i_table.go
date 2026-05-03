// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

import (
	"context"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
)

// ITable represents the interface for LanceDB table operations.
// This interface abstracts the Table struct methods to enable better testing,
// mocking, and decoupling in applications using LanceDB.
type ITable interface {
	// Name returns the name of the table
	Name() string

	// IsOpen returns true if the table is currently open and available for operations
	IsOpen() bool

	// Close closes the table and releases associated resources
	Close() error

	// Schema returns the Arrow schema of the table
	Schema(ctx context.Context) (*arrow.Schema, error)

	// Add inserts a single Arrow Record into the table
	// Deprecated: Use AddRecords for better performance with batch processing
	Add(ctx context.Context, record arrow.Record, options *AddDataOptions) error

	// AddRecords efficiently adds multiple records using Arrow IPC batch processing
	AddRecords(ctx context.Context, records []arrow.Record, options *AddDataOptions) error

	// Query creates a new query builder for constructing complex queries
	Query() IQueryBuilder

	// VectorQuery creates a new vector query builder for similarity searches on the specified column
	VectorQuery(column string, vector []float32) IVectorQueryBuilder

	// Count returns the total number of rows in the table
	Count(ctx context.Context) (int64, error)

	// Version returns the current version number of the table
	Version(ctx context.Context) (int, error)

	// Update modifies existing records in the table based on the given filter
	// The updates parameter is a map where keys are column names and values are the new values
	Update(ctx context.Context, filter string, updates map[string]interface{}) error

	// Delete removes records from the table that match the given filter
	Delete(ctx context.Context, filter string) error

	// MergeInsert returns a builder for a merge_insert (upsert) operation keyed
	// on one or more columns. Configure the builder and call Execute to run.
	MergeInsert(on []string) IMergeInsertBuilder

	// CreateIndex creates an index on the specified columns using the given index type
	CreateIndex(ctx context.Context, columns []string, indexType IndexType) error

	// CreateIndexWithName creates an index with a custom name on the specified columns
	CreateIndexWithName(ctx context.Context, columns []string, indexType IndexType, name string) error

	// CreateIndexWithParams creates an index with full per-type tuning
	// (IVF partition/PQ/HNSW/FTS parameters) plus top-level options
	// (name / replace / wait timeout). Unset fields fall back to the
	// backend default. Passing nil opts is equivalent to zero-valued
	// CreateIndexOptions (no name override, replace=false, no wait).
	CreateIndexWithParams(ctx context.Context, columns []string, indexType IndexType, params IndexParams, opts *CreateIndexOptions) error

	// DropIndex removes the named index from the table. Returns an error
	// if the index does not exist or the backend operation fails — IF
	// EXISTS semantics are the caller's responsibility.
	DropIndex(ctx context.Context, name string) error

	// GetAllIndexes returns information about all indexes present on the table
	GetAllIndexes(ctx context.Context) ([]IndexInfo, error)

	// Retrieve statistics about an index
	IndexStats(ctx context.Context, indexName string) (*IndexStatistics, error)

	// WaitForIndex blocks until the named indexes have finished building
	// or the timeout elapses. An empty names slice waits for every index
	// on the table. A zero timeout means "no upper bound" — rely on ctx
	// cancellation to abort early. Returns an error on timeout, missing
	// index, or backend failure.
	WaitForIndex(ctx context.Context, names []string, timeout time.Duration) error

	// Select executes a query with the provided configuration and returns the results
	Select(ctx context.Context, config QueryConfig) ([]map[string]interface{}, error)

	// SelectWithColumns returns all records with only the specified columns
	SelectWithColumns(ctx context.Context, columns []string) ([]map[string]interface{}, error)

	// SelectWithFilter returns records that match the given filter condition
	SelectWithFilter(ctx context.Context, filter string) ([]map[string]interface{}, error)

	// VectorSearch performs vector similarity search on the specified column
	// Returns the k most similar records to the given vector
	VectorSearch(ctx context.Context, column string, vector []float32, k int) ([]map[string]interface{}, error)

	// VectorSearchWithFilter performs vector similarity search with an additional filter condition
	VectorSearchWithFilter(ctx context.Context, column string, vector []float32, k int, filter string) ([]map[string]interface{}, error)

	// FullTextSearch performs full-text search on the specified column
	FullTextSearch(ctx context.Context, column string, query string) ([]map[string]interface{}, error)

	// FullTextSearchWithFilter performs full-text search with an additional filter condition
	FullTextSearchWithFilter(ctx context.Context, column string, query string, filter string) ([]map[string]interface{}, error)

	// SelectWithLimit returns a limited number of records with optional offset
	SelectWithLimit(ctx context.Context, limit int, offset int) ([]map[string]interface{}, error)

	// Optimize the on-disk data and indices for better performance.
	// Equivalent to OptimizeWithAction(ctx, OptimizeAction{Kind: OptimizeAll}).
	Optimize(ctx context.Context) (*OptimizeStats, error)

	// OptimizeWithAction runs a configurable optimize action — Compact,
	// Prune, Index, or All. Use this when you need to control which
	// sub-pass runs (e.g. only Prune to reclaim disk after deletions)
	// or to tune per-action parameters.
	OptimizeWithAction(ctx context.Context, action OptimizeAction) (*OptimizeStats, error)
}

// ITableUpdateExpr is an optional capability extension layered on top of
// ITable. Backends that support lancedb's raw-SQL-expression update
// builder implement it; backends that don't are unaffected.
//
// Kept out of ITable so adding the capability to a downstream backend
// (or removing it later) is not a source-breaking change for existing
// ITable mocks/stubs. Callers detect the capability with a type
// assertion:
//
//	if u, ok := table.(contracts.ITableUpdateExpr); ok {
//	    res, err := u.UpdateExpr(ctx, filter, assignments)
//	}
//
// The shipped *internal.Table implements this interface.
type ITableUpdateExpr interface {
	// UpdateExpr is a thin pass-through to lancedb's Table::update
	// builder that exposes raw SQL expressions per column and an
	// optional filter.
	//
	// Differences from ITable.Update:
	//   - filter == "" updates every row (no WHERE).
	//   - assignments[i].Expr is forwarded verbatim — the caller quotes
	//     string literals (`'foo'`) and formats vector literals
	//     (`[1.0, 2.0, ...]`). This unlocks expressions the Update path
	//     auto-quotes away, e.g. `counter + 1`, `upper(name)`,
	//     `coalesce(other, 0)`.
	//   - Returns rows_updated and the new commit version.
	//
	// An empty assignments slice is rejected — UPDATE with no SET is
	// semantically a no-op and almost always a caller bug.
	UpdateExpr(ctx context.Context, filter string, assignments []UpdateAssignment) (*UpdateResult, error)
}

// AddDataOptions configures how data is added to a Table
type AddDataOptions struct {
	Mode WriteMode
}

// WriteMode specifies how data should be written to a Table
type WriteMode int

const (
	WriteModeAppend WriteMode = iota
	WriteModeOverwrite
)

// ITableTimeTravel is an optional capability extension layered on top
// of ITable. It exposes lancedb's version-history surface — listing
// past versions, pinning the table to a specific version (read-only
// view), restoring that version as the new latest, plus tag CRUD that
// gives stable human-readable references to versions.
//
// Kept out of ITable so adding the capability to a downstream backend
// (or removing it later) is not a source-breaking change for existing
// ITable mocks/stubs. Callers detect the capability with a type
// assertion:
//
//	if tt, ok := table.(contracts.ITableTimeTravel); ok {
//	    versions, err := tt.ListVersions(ctx)
//	}
//
// Semantic notes that mirror lancedb::Table:
//
//   - Checkout / CheckoutTag pin the table to a snapshot. Reads see
//     that snapshot; writes are rejected until the table is brought
//     back with CheckoutLatest or promoted with Restore.
//   - Restore promotes the currently checked-out version to a new
//     latest manifest (i.e. "make this the live data"). It errors when
//     the table is not in a checked-out state. There is no
//     restore(version) overload — do Checkout(N) -> Restore() in two
//     steps.
//   - Tags are pure metadata: creating, deleting, or moving a tag does
//     not produce a new dataset version.
//   - Tag-protected versions interact with prune via
//     OptimizePrune.ErrorIfTaggedOldVersions — see types.go.
//
// The shipped *internal.Table implements this interface.
type ITableTimeTravel interface {
	// ListVersions returns the full version history known to the
	// dataset. Order matches the backend's response.
	ListVersions(ctx context.Context) ([]VersionInfo, error)

	// Checkout pins the table to the given version. Subsequent reads
	// see that snapshot. Writes are rejected until the pin is dropped
	// with CheckoutLatest or promoted with Restore.
	Checkout(ctx context.Context, version uint64) error

	// CheckoutTag pins the table to the version referenced by the
	// given tag. Same constraints as Checkout.
	CheckoutTag(ctx context.Context, tag string) error

	// CheckoutLatest drops any prior checkout pin and resumes
	// tracking the latest manifest.
	CheckoutLatest(ctx context.Context) error

	// Restore promotes the currently checked-out version to a new
	// latest manifest. Errors when the table is not in a checked-out
	// state. After Restore the table is no longer pinned.
	Restore(ctx context.Context) error

	// TagList returns every tag on the table, keyed by tag name.
	TagList(ctx context.Context) (map[string]TagInfo, error)

	// TagGetVersion resolves a tag to its pinned version. Errors when
	// the tag does not exist.
	TagGetVersion(ctx context.Context, tag string) (uint64, error)

	// TagCreate creates a new tag pointing at the given version.
	// Errors when the tag already exists or the version is unknown.
	TagCreate(ctx context.Context, tag string, version uint64) error

	// TagDelete deletes a tag. Errors when the tag does not exist.
	TagDelete(ctx context.Context, tag string) error

	// TagUpdate moves an existing tag to a new version. Errors when
	// the tag does not exist or the version is unknown.
	TagUpdate(ctx context.Context, tag string, version uint64) error
}

// ITableSchemaEvolve is an optional capability extension layered on
// top of ITable. It exposes lancedb's schema-evolution surface — adding
// derived columns, renaming columns, toggling nullability, and
// dropping columns. Each operation is a metadata commit (drop_columns
// in particular only updates the manifest; the on-disk bytes are
// reclaimed on the next compaction).
//
// Kept out of ITable so adding the capability to a downstream backend
// (or removing it later) is not a source-breaking change for existing
// ITable mocks/stubs. Callers detect the capability with a type
// assertion:
//
//	if se, ok := table.(contracts.ITableSchemaEvolve); ok {
//	    res, err := se.AddColumns(ctx, []NewColumnTransform{
//	        {Name: "score_x2", Expression: "score * 2"},
//	    })
//	}
//
// Scope (v1):
//   - AddColumns supports only the SqlExpressions transform
//     (lance::dataset::NewColumnTransform::SqlExpressions). The
//     BatchUDF / Stream / Reader / AllNulls variants are not exposed
//     because they require closures or full Arrow IPC plumbing across
//     the FFI boundary.
//   - AlterColumns supports rename and nullable changes only. The
//     data_type cast variant is deliberately deferred — it requires
//     Arrow DataType serialization. Until then, callers needing a
//     cast must AddColumns(<new-with-cast-expr>) → DropColumns(<old>).
//   - DropColumns is the full surface.
//
// The shipped *internal.Table implements this interface.
type ITableSchemaEvolve interface {
	// AddColumns adds new columns to the table by evaluating SQL
	// expressions over existing rows. Returns the new commit version.
	// An empty transforms slice is rejected — adding zero columns is
	// a no-op and almost always a caller bug.
	AddColumns(ctx context.Context, transforms []NewColumnTransform) (uint64, error)

	// AlterColumns renames columns and/or toggles their nullability.
	// Returns the new commit version. Each entry must change at least
	// one attribute; a no-op alteration is rejected. An empty
	// alterations slice is rejected.
	AlterColumns(ctx context.Context, alterations []ColumnAlteration) (uint64, error)

	// DropColumns removes the named columns from the table. The
	// underlying bytes are reclaimed on the next OptimizeCompact.
	// Returns the new commit version. An empty names slice is
	// rejected; empty/whitespace-only names are rejected per-entry.
	DropColumns(ctx context.Context, names []string) (uint64, error)
}
