// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

import (
	"encoding/json"
	"time"
)

// The types in this file are additive capabilities introduced after the
// published v0.1.2 API. They deliberately do not change the field layout or
// tags of QueryConfig, VectorSearch, ConnectionOptions, StorageOptions,
// IndexInfo, or any other published type.

// QueryConfigV2 is the additive query configuration used by the optional
// ITableQueryV2 interface. It carries the query controls added after v0.1.2
// without changing the published QueryConfig shape.
type QueryConfigV2 struct {
	Columns      []string
	Where        string
	Limit        *int
	Offset       *int
	VectorSearch *VectorSearchV2
	FTSSearch    *FTSSearch

	// WithRowID adds the internal _rowid column to the result.
	WithRowID bool
	// FastSearch skips rows not yet covered by an index.
	FastSearch bool
	// Postfilter evaluates WHERE after the candidate set is built.
	Postfilter bool
	// Reranker installs a reranker on the query.
	Reranker *RerankerConfig
}

// VectorSearchV2 is the additive vector-search configuration. DistanceType is
// optional so an omitted metric remains distinct from L2.
type VectorSearchV2 struct {
	Column            string
	Vector            []float32
	K                 int
	DistanceType      *DistanceType
	Nprobes           *int
	RefineFactor      *uint32
	Ef                *int
	BypassVectorIndex bool

	// FullTextQuery, when non-empty alongside Vector, turns the query into a
	// hybrid search. FullTextColumn may be empty to let LanceDB discover the
	// indexed FTS column.
	FullTextQuery  string
	FullTextColumn string
}

// IndexParams carries per-type tuning knobs for CreateIndexWithParams. Unset
// numeric fields are nil; unset optional distance uses nil.
type IndexParams struct {
	NumPartitions       *uint32
	SampleRate          *uint32
	MaxIterations       *uint32
	TargetPartitionSize *uint32
	NumSubVectors       *uint32
	NumBits             *uint32
	M                   *uint32
	EfConstruction      *uint32
	DistanceType        *DistanceType

	FtsLanguage        string
	FtsWithPosition    *bool
	FtsStem            *bool
	FtsRemoveStopWords *bool
	FtsLowerCase       *bool
	FtsASCIIFolding    *bool
	FtsBaseTokenizer   string
	FtsMaxTokenLength  *uint32
	FtsNgramMinLength  *uint32
	FtsNgramMaxLength  *uint32
	FtsNgramPrefixOnly *bool
}

// CreateIndexOptions carries the top-level index builder knobs.
type CreateIndexOptions struct {
	Name        string
	Replace     bool
	WaitTimeout time.Duration
}

// IndexStatistics is the rich index statistics view. Loss is intentionally
// absent: the Rust SDK removed training loss, and this type was never part of
// the published v0.1.2 API.
type IndexStatistics struct {
	NumIndexedRows   int64
	NumUnindexedRows int64
	IndexType        string
	DistanceType     *string
	NumIndices       *int
}

// RerankerKind identifies the reranker to apply to a query's results.
type RerankerKind int

const (
	RerankerNone RerankerKind = iota
	RerankerRRF
)

// NormalizeMethod controls how a reranker combines scores across modalities.
type NormalizeMethod int

const (
	NormalizeDefault NormalizeMethod = iota
	NormalizeScore
	NormalizeRank
)

// RerankerConfig describes how to rerank query results.
type RerankerConfig struct {
	Kind RerankerKind
	RRFK float32
	Norm NormalizeMethod
}

// MarshalJSON preserves the wire shape introduced on Go main for callers that
// serialize a reranker configuration themselves. The native path does not use
// this encoding, but it remains part of the additive public type contract.
func (rc *RerankerConfig) MarshalJSON() ([]byte, error) {
	if rc == nil || rc.Kind == RerankerNone {
		return []byte("null"), nil
	}
	var wire struct {
		Kind string   `json:"kind"`
		K    *float32 `json:"k,omitempty"`
		Norm string   `json:"norm,omitempty"`
	}
	switch rc.Kind {
	case RerankerRRF:
		wire.Kind = "rrf"
	}
	if rc.RRFK > 0 {
		k := rc.RRFK
		wire.K = &k
	}
	switch rc.Norm {
	case NormalizeScore:
		wire.Norm = "score"
	case NormalizeRank:
		wire.Norm = "rank"
	}
	return json.Marshal(wire)
}

// CompactionMetrics reports what compact did.
type CompactionMetrics struct {
	FragmentsRemoved *int64
	FragmentsAdded   *int64
	FilesRemoved     *int64
	FilesAdded       *int64
}

// RemovalStats reports what prune removed.
type RemovalStats struct {
	BytesRemoved *int64
	OldVersions  *int64
}

// OptimizeActionKind selects an optimize sub-action.
type OptimizeActionKind int

const (
	OptimizeAll OptimizeActionKind = iota
	OptimizeCompact
	OptimizePrune
	OptimizeIndex
)

// CompactionParams carries CompactionOptions for OptimizeCompact.
type CompactionParams struct {
	TargetRowsPerFragment         *uint64
	MaxRowsPerGroup               *uint64
	MaxBytesPerFile               *uint64
	MaterializeDeletions          *bool
	MaterializeDeletionsThreshold *float32
	NumThreads                    *uint64
	BatchSize                     *uint64
}

// PruneParams carries OptimizeAction::Prune options. OlderThan is converted
// to whole seconds, with positive sub-second values floored to one second to
// preserve the v0.1.2 behavior.
type PruneParams struct {
	OlderThan                time.Duration
	DeleteUnverified         *bool
	ErrorIfTaggedOldVersions *bool
}

// OptimizeAction selects an optimize sub-action and its parameters.
type OptimizeAction struct {
	Kind       OptimizeActionKind
	Compaction CompactionParams
	Prune      PruneParams
}

// OptimizeStats reports the statistics of one optimize action.
type OptimizeStats struct {
	Compaction *CompactionMetrics
	Prune      *RemovalStats
}

// MergeResult mirrors the SDK merge result.
type MergeResult struct {
	Version         uint64
	NumInsertedRows uint64
	NumUpdatedRows  uint64
	NumDeletedRows  uint64
	NumAttempts     uint32
}

// UpdateAssignment is one SET clause. Expr is forwarded verbatim as a SQL
// expression; the caller quotes string literals with single quotes.
type UpdateAssignment struct {
	Column string
	Expr   string
}

// UpdateResult reports an update's affected rows and commit version.
type UpdateResult struct {
	RowsUpdated uint64
	Version     uint64
}

// VersionInfo describes one dataset version.
type VersionInfo struct {
	Version   uint64
	Timestamp time.Time
	Metadata  map[string]string
}

// TagInfo describes one tag.
type TagInfo struct {
	Version      uint64
	ManifestSize uint64
	Branch       string
}

// NewColumnTransform describes one SQL-derived new column.
type NewColumnTransform struct {
	Name       string
	Expression string
}

// ColumnAlteration describes a rename or nullability change.
type ColumnAlteration struct {
	Path     string
	Rename   *string
	Nullable *bool
}
