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
