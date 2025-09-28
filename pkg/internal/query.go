// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../../lib/darwin_amd64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../../lib/darwin_arm64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../../lib/linux_amd64/liblancedb_go.a
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/../../lib/linux_arm64/liblancedb_go.a
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/../../lib/windows_amd64/liblancedb_go.a
#include "lancedb.h"
*/
import "C"

import (
	"fmt"

	"github.com/apache/arrow/go/v17/arrow"

	lancedb "github.com/lancedb/lancedb-go/pkg/contracts"
)

// QueryBuilder provides a fluent interface for building queries
type QueryBuilder struct {
	table   *Table
	filters []string
	limit   int
}

var _ lancedb.IQueryBuilder = (*QueryBuilder)(nil)
var _ lancedb.IVectorQueryBuilder = (*VectorQueryBuilder)(nil)

// VectorQueryBuilder extends QueryBuilder for vector similarity searches
type VectorQueryBuilder struct {
	QueryBuilder
	vector []float32
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

// Execute executes the query and returns results
func (q *QueryBuilder) Execute() ([]arrow.Record, error) {
	if q.table.connection.closed {
		return nil, fmt.Errorf("table is closed")
	}

	// This is a placeholder implementation
	// In practice, we'd need to build and execute the query through the Rust layer
	return nil, fmt.Errorf("query execution not yet implemented")
}

// Filter adds a filter condition to the vector query
func (vq *VectorQueryBuilder) Filter(condition string) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.Filter(condition)
	return vq
}

// Limit sets the maximum number of results to return
func (vq *VectorQueryBuilder) Limit(limit int) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.Limit(limit)
	return vq
}

// DistanceType sets the distance metric for vector search
func (vq *VectorQueryBuilder) DistanceType(_ lancedb.DistanceType) lancedb.IVectorQueryBuilder {
	// Store distance type for later use
	return vq
}

// Execute executes the vector search query and returns results
func (vq *VectorQueryBuilder) Execute() ([]arrow.Record, error) {
	if vq.table.connection.closed {
		return nil, fmt.Errorf("table is closed")
	}

	// Placeholder implementation - vector query execution not yet fully implemented in C API
	// This is a temporary workaround until the C API types are properly exported
	return nil, fmt.Errorf("vector query execution not yet implemented - C API types need to be fixed")
}

// ExecuteAsync executes the query asynchronously
func (q *QueryBuilder) ExecuteAsync() (<-chan []arrow.Record, <-chan error) {
	resultChan := make(chan []arrow.Record, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		results, err := q.Execute()
		if err != nil {
			errorChan <- err
			return
		}

		resultChan <- results
	}()

	return resultChan, errorChan
}

// ExecuteAsync executes the vector query asynchronously
func (vq *VectorQueryBuilder) ExecuteAsync() (<-chan []arrow.Record, <-chan error) {
	resultChan := make(chan []arrow.Record, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		results, err := vq.Execute()
		if err != nil {
			errorChan <- err
			return
		}

		resultChan <- results
	}()

	return resultChan, errorChan
}

// ApplyOptions applies query options to the builder
func (q *QueryBuilder) ApplyOptions(options *lancedb.QueryOptions) lancedb.IQueryBuilder {
	if options != nil {
		if options.MaxResults > 0 {
			q.Limit(options.MaxResults)
		}
		// Store other options for later use in query execution
	}
	return q
}

// ApplyOptions applies query options to the vector query builder
func (vq *VectorQueryBuilder) ApplyOptions(options *lancedb.QueryOptions) lancedb.IVectorQueryBuilder {
	vq.QueryBuilder.ApplyOptions(options)
	return vq
}
