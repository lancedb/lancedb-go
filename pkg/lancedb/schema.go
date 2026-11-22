// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
)

// NewSchema creates a new schema from an Arrow schema.
func NewSchema(schema *arrow.Schema) (contracts.ISchema, error) {
	return internal.NewSchema(schema)
}

// NewSchemaBuilder returns a new schema builder.
func NewSchemaBuilder() contracts.ISchemaBuilder {
	return internal.NewSchemaBuilder()
}
