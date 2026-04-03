// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

type IQueryBuilder interface {
	Filter(condition string) IQueryBuilder
	Limit(limit int) IQueryBuilder
	Columns(columns []string) IQueryBuilder
	Offset(offset int) IQueryBuilder
	Execute() ([]map[string]interface{}, error)
	ExecuteAsync() (<-chan []map[string]interface{}, <-chan error)
	ApplyOptions(options *QueryOptions) IQueryBuilder
}

type IVectorQueryBuilder interface {
	Filter(condition string) IVectorQueryBuilder
	Limit(limit int) IVectorQueryBuilder
	Columns(columns []string) IVectorQueryBuilder
	Execute() ([]map[string]interface{}, error)
	ExecuteAsync() (<-chan []map[string]interface{}, <-chan error)
	ApplyOptions(options *QueryOptions) IVectorQueryBuilder
}

// QueryOptions provides additional configuration for queries
type QueryOptions struct {
	MaxResults        int
	UseFullPrecision  bool
	BypassVectorIndex bool
}
