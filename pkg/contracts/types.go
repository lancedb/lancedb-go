package contracts

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
