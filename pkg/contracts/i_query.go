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
	Execute(ctx context.Context) (arrow.Record, error)
	ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IQueryBuilder
}

type IVectorQueryBuilder interface {
	Filter(condition string) IVectorQueryBuilder
	Limit(limit int) IVectorQueryBuilder
	Columns(columns []string) IVectorQueryBuilder
	DistanceType(dt DistanceType) IVectorQueryBuilder
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
