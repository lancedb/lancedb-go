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
}

// VectorSearch represents vector similarity search parameters
type VectorSearch struct {
	Column string    `json:"column"`
	Vector []float32 `json:"vector"`
	K      int       `json:"k"`
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
