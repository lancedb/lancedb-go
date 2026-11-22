package contracts

import "github.com/apache/arrow/go/v17/arrow"

type IQueryBuilder interface {
	Filter(condition string) IQueryBuilder
	Limit(limit int) IQueryBuilder
	Execute() ([]arrow.Record, error)
	ExecuteAsync() (<-chan []arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IQueryBuilder
}

type IVectorQueryBuilder interface {
	Filter(condition string) IVectorQueryBuilder
	Limit(limit int) IVectorQueryBuilder
	DistanceType(_ DistanceType) IVectorQueryBuilder
	Execute() ([]arrow.Record, error)
	ExecuteAsync() (<-chan []arrow.Record, <-chan error)
	ApplyOptions(options *QueryOptions) IVectorQueryBuilder
}

// QueryOptions provides additional configuration for queries
type QueryOptions struct {
	MaxResults        int
	UseFullPrecision  bool
	BypassVectorIndex bool
}

// DistanceType represents vector distance metrics
type DistanceType int

const (
	DistanceTypeL2 DistanceType = iota
	DistanceTypeCosine
	DistanceTypeDot
	DistanceTypeHamming
)
