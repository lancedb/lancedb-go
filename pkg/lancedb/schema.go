package lancedb

import (
	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
)

// NewSchema creates a new schema from Arrow schema
func NewSchema(schema *arrow.Schema) (contracts.ISchema, error) {
	return internal.NewSchema(schema)
}

func NewSchemaBuilder() contracts.ISchemaBuilder {
	return internal.NewSchemaBuilder()
}
