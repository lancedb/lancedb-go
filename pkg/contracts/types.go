package contracts

import (
	"encoding/json"
	"time"
)

// IndexType represents the type of index to create
type IndexType int

const (
	IndexTypeAuto IndexType = iota
	IndexTypeIvfPq
	IndexTypeIvfFlat
	IndexTypeHnswPq
	IndexTypeHnswSq
	IndexTypeBTree
	IndexTypeBitmap
	IndexTypeLabelList
	IndexTypeFts
)

// DistanceType represents the distance metric for vector similarity search
type DistanceType int

const (
	DistanceTypeUnspecified DistanceType = iota // use backend default
	DistanceTypeL2                              // Euclidean distance
	DistanceTypeCosine                          // Cosine similarity
	DistanceTypeDot                             // Dot product
)

// IndexInfo represents information about an index on a table
type IndexInfo struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	IndexType string   `json:"index_type"`
}

// IndexParams carries per-type tuning knobs for CreateIndexWithParams.
// All fields are optional — pointer fields treat nil as "backend default",
// string fields treat empty as "unset". A field that does not apply to the
// chosen IndexType (e.g. M on a BTree index) is silently ignored on the
// Rust side.
type IndexParams struct {
	// IVF-family common
	NumPartitions       *uint32 `json:"num_partitions,omitempty"`
	SampleRate          *uint32 `json:"sample_rate,omitempty"`
	MaxIterations       *uint32 `json:"max_iterations,omitempty"`
	TargetPartitionSize *uint32 `json:"target_partition_size,omitempty"`
	// PQ-specific (IvfPq, IvfRq, IvfHnswPq)
	NumSubVectors *uint32 `json:"num_sub_vectors,omitempty"`
	NumBits       *uint32 `json:"num_bits,omitempty"`
	// HNSW-specific
	M              *uint32 `json:"m,omitempty"`
	EfConstruction *uint32 `json:"ef_construction,omitempty"`
	// Distance metric for vector indices. Leave Unspecified for the
	// backend default (L2).
	DistanceType DistanceType `json:"-"`

	// FTS-specific
	FtsLanguage        string  `json:"language,omitempty"`
	FtsWithPosition    *bool   `json:"with_position,omitempty"`
	FtsStem            *bool   `json:"stem,omitempty"`
	FtsRemoveStopWords *bool   `json:"remove_stop_words,omitempty"`
	FtsLowerCase       *bool   `json:"lower_case,omitempty"`
	FtsASCIIFolding    *bool   `json:"ascii_folding,omitempty"`
	FtsBaseTokenizer   string  `json:"base_tokenizer,omitempty"`
	FtsMaxTokenLength  *uint32 `json:"max_token_length,omitempty"`
	FtsNgramMinLength  *uint32 `json:"ngram_min_length,omitempty"`
	FtsNgramMaxLength  *uint32 `json:"ngram_max_length,omitempty"`
	FtsNgramPrefixOnly *bool   `json:"ngram_prefix_only,omitempty"`
}

// CreateIndexOptions carries the top-level IndexBuilder knobs that live
// outside of the per-type params (name, replace, wait_timeout).
type CreateIndexOptions struct {
	// Name overrides the default lancedb-chosen index name.
	Name string
	// Replace controls whether an existing index on the same columns
	// with the same name is replaced. Matches lancedb::IndexBuilder::replace.
	Replace bool
	// WaitTimeout tells the backend to block until index build completes
	// or the timeout elapses. Zero leaves the default (no wait).
	WaitTimeout time.Duration
}

// RerankerKind identifies the reranker to apply to a query's results. The
// upstream lancedb v0.24.0 ships RRF as its only built-in; this enum
// leaves room for future kinds without breaking callers.
type RerankerKind int

const (
	// RerankerNone leaves the query un-reranked.
	RerankerNone RerankerKind = iota
	// RerankerRRF is Reciprocal Rank Fusion. Good default for hybrid
	// vector+FTS queries.
	RerankerRRF
)

// NormalizeMethod maps to lancedb::rerankers::NormalizeMethod. Controls
// how the reranker combines scores across modalities.
type NormalizeMethod int

const (
	// NormalizeDefault leaves the reranker's own default behaviour.
	NormalizeDefault NormalizeMethod = iota
	// NormalizeScore normalises by raw score.
	NormalizeScore
	// NormalizeRank normalises by rank (typical for RRF).
	NormalizeRank
)

// RerankerConfig describes how to rerank query results. Kind selects the
// reranker; the remaining fields are per-kind. Leave RerankerNone to skip
// reranking.
type RerankerConfig struct {
	Kind RerankerKind
	// RRFK maps to lancedb::rerankers::RRFReranker::new(k). Defaults to
	// 60.0 when zero and Kind == RerankerRRF (matches upstream).
	RRFK float32
	// Norm sets the normalization method for the reranker.
	Norm NormalizeMethod
}

// MarshalJSON emits the wire shape consumed by the Rust FFI
// ({"kind":"rrf","k":...,"norm":...}). RerankerNone marshals to null so
// omitempty on the parent field drops the section entirely.
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

// IndexStatistics represents statistics about an index
type IndexStatistics struct {
	NumIndexedRows   int64    `json:"num_indexed_rows"`
	NumUnindexedRows int64    `json:"num_unindexed_rows"`
	IndexType        string   `json:"index_type"`
	DistanceType     *string  `json:"distance_type,omitempty"`
	NumIndices       *int     `json:"num_indices,omitempty"`
	Loss             *float64 `json:"loss,omitempty"`
}

// QueryConfig represents the configuration for a select query
type QueryConfig struct {
	Columns      []string      `json:"columns,omitempty"`
	Where        string        `json:"where,omitempty"`
	Limit        *int          `json:"limit,omitempty"`
	Offset       *int          `json:"offset,omitempty"`
	VectorSearch *VectorSearch `json:"vector_search,omitempty"`
	FTSSearch    *FTSSearch    `json:"fts_search,omitempty"`

	// WithRowID adds the internal _rowid column to the result. Maps to
	// lancedb's QueryBase::with_row_id().
	WithRowID bool `json:"with_row_id,omitempty"`
	// FastSearch skips rows not yet covered by an index. Maps to
	// lancedb's QueryBase::fast_search(). Weak consistency tradeoff.
	FastSearch bool `json:"fast_search,omitempty"`
	// Postfilter switches WHERE evaluation to run after the vector/FTS
	// candidate set is materialised. Default is prefilter. Maps to
	// QueryBase::postfilter().
	Postfilter bool `json:"postfilter,omitempty"`

	// Reranker attaches a reranker to the query. Nil leaves the backend
	// default (no reranker on single-channel queries; automatic RRF on
	// hybrid nearest_to + full_text_search queries).
	Reranker *RerankerConfig `json:"reranker,omitempty"`
}

// VectorSearch represents vector similarity search parameters
type VectorSearch struct {
	Column       string    `json:"column"`
	Vector       []float32 `json:"vector"`
	K            int       `json:"k"`
	DistanceType *string   `json:"distance_type,omitempty"`

	// Nprobes is the IVF partition scan count. Larger => higher recall,
	// higher latency. Maps to VectorQuery::nprobes().
	Nprobes *int `json:"nprobes,omitempty"`
	// RefineFactor multiplies the k passed to the first IVF stage. Maps
	// to VectorQuery::refine_factor().
	RefineFactor *uint32 `json:"refine_factor,omitempty"`
	// Ef is the HNSW candidate list size. Larger => higher recall.
	// Maps to VectorQuery::ef(). HNSW indices only.
	Ef *int `json:"ef,omitempty"`
	// BypassVectorIndex forces a flat (exhaustive) scan instead of the
	// trained index. Maps to VectorQuery::bypass_vector_index().
	BypassVectorIndex bool `json:"bypass_vector_index,omitempty"`

	// FullTextQuery, when non-empty alongside Vector, turns the query
	// into a hybrid search: lancedb runs both the dense nearest_to and
	// an FTS pass and fuses the results with the configured reranker
	// (RRF by default). The table must have an FTS index on the
	// matching text column; use CreateIndexWithParams(FTS, ...) first.
	FullTextQuery string `json:"full_text_query,omitempty"`
	// FullTextColumn optionally pins the FTS column. Empty lets lancedb
	// pick the one FTS-indexed column on the table.
	FullTextColumn string `json:"full_text_column,omitempty"`
}

// FTSSearch represents full-text search parameters
type FTSSearch struct {
	Column string `json:"column"`
	Query  string `json:"query"`
}

// QueryResult represents the result of a select query
type QueryResult struct {
	Rows []map[string]interface{} `json:"rows"`
}

// CompactionMetrics represents statistics about the optimization
type CompactionMetrics struct {
	FragmentsRemoved *int64 `json:"fragments_removed,omitempty"`
	FragmentsAdded   *int64 `json:"fragments_added,omitempty"`
	FilesRemoved     *int64 `json:"files_removed,omitempty"`
	FilesAdded       *int64 `json:"files_added,omitempty"`
}

// RemovalStats represents stats of the file compaction
type RemovalStats struct {
	BytesRemoved *int64 `json:"bytes_removed,omitempty"`
	OldVersions  *int64 `json:"old_versions,omitempty"`
}

// OptimizeStats represents stats of the version pruning
type OptimizeStats struct {
	Compaction *CompactionMetrics `json:"compaction,omitempty"`
	Prune      *RemovalStats      `json:"prune,omitempty"`
}

// MergeResult's JSON tags mirror lancedb::table::MergeResult's serde form.
type MergeResult struct {
	Version         uint64 `json:"version"`
	NumInsertedRows uint64 `json:"num_inserted_rows"`
	NumUpdatedRows  uint64 `json:"num_updated_rows"`
	NumDeletedRows  uint64 `json:"num_deleted_rows"`
	NumAttempts     uint32 `json:"num_attempts"`
}
