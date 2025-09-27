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
