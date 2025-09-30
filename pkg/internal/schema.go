// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#include "lancedb.h"
*/
import "C"

import (
	"fmt"

	"github.com/apache/arrow/go/v17/arrow"

	lancedb "github.com/lancedb/lancedb-go/pkg/contracts"
)

// Schema represents a LanceDB table schema
type Schema struct {
	schema *arrow.Schema
}

var _ lancedb.ISchema = (*Schema)(nil)

// Field represents a schema field
type Field struct {
	Name     string
	DataType arrow.DataType
	Nullable bool
	Metadata arrow.Metadata
}

// NewSchema creates a new schema from Arrow schema
func NewSchema(schema *arrow.Schema) (lancedb.ISchema, error) {
	return &Schema{
		schema: schema,
	}, nil
}

// SchemaBuilder provides a fluent interface for building schemas
type SchemaBuilder struct {
	fields []arrow.Field
}

var _ lancedb.ISchemaBuilder = (*SchemaBuilder)(nil)

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder() lancedb.ISchemaBuilder {
	return &SchemaBuilder{
		fields: make([]arrow.Field, 0),
	}
}

// AddField adds a regular field to the schema
func (sb *SchemaBuilder) AddField(name string, dataType arrow.DataType, nullable bool) lancedb.ISchemaBuilder {
	field := arrow.Field{
		Name:     name,
		Type:     dataType,
		Nullable: nullable,
	}
	sb.fields = append(sb.fields, field)
	return sb
}

// AddVectorField adds a vector field to the schema
func (sb *SchemaBuilder) AddVectorField(name string, dimension int, dataType lancedb.VectorDataType, nullable bool) lancedb.ISchemaBuilder {
	var itemType arrow.DataType
	switch dataType {
	case lancedb.VectorDataTypeFloat16:
		itemType = arrow.PrimitiveTypes.Float32
	case lancedb.VectorDataTypeFloat32:
		itemType = arrow.PrimitiveTypes.Float32
	case lancedb.VectorDataTypeFloat64:
		itemType = arrow.PrimitiveTypes.Float64
	default:
		itemType = arrow.PrimitiveTypes.Float32
	}

	vectorType := arrow.FixedSizeListOf(int32(dimension), itemType)
	field := arrow.Field{
		Name:     name,
		Type:     vectorType,
		Nullable: nullable,
	}
	sb.fields = append(sb.fields, field)
	return sb
}

// AddInt32Field adds an int32 field to the schema
func (sb *SchemaBuilder) AddInt32Field(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.PrimitiveTypes.Int32, nullable)
}

// AddInt64Field adds an int64 field to the schema
func (sb *SchemaBuilder) AddInt64Field(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.PrimitiveTypes.Int64, nullable)
}

// AddFloat32Field adds a float32 field to the schema
func (sb *SchemaBuilder) AddFloat32Field(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.PrimitiveTypes.Float32, nullable)
}

// AddFloat64Field adds a float64 field to the schema
func (sb *SchemaBuilder) AddFloat64Field(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.PrimitiveTypes.Float64, nullable)
}

// AddStringField adds a string field to the schema
func (sb *SchemaBuilder) AddStringField(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.BinaryTypes.String, nullable)
}

// AddBinaryField adds a binary field to the schema
func (sb *SchemaBuilder) AddBinaryField(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.BinaryTypes.Binary, nullable)
}

// AddBooleanField adds a boolean field to the schema
func (sb *SchemaBuilder) AddBooleanField(name string, nullable bool) lancedb.ISchemaBuilder {
	return sb.AddField(name, arrow.FixedWidthTypes.Boolean, nullable)
}

// AddTimestampField adds a timestamp field to the schema
func (sb *SchemaBuilder) AddTimestampField(name string, unit arrow.TimeUnit, nullable bool) lancedb.ISchemaBuilder {
	timestampType := &arrow.TimestampType{Unit: unit}
	return sb.AddField(name, timestampType, nullable)
}

// Build creates the final schema
func (sb *SchemaBuilder) Build() (lancedb.ISchema, error) {
	arrowSchema := arrow.NewSchema(sb.fields, nil)
	return NewSchema(arrowSchema)
}

// Fields returns the fields in the schema
func (s *Schema) Fields() []arrow.Field {
	if s.schema != nil {
		return s.schema.Fields()
	}
	return nil
}

// NumFields returns the number of fields in the schema
func (s *Schema) NumFields() int {
	if s.schema != nil {
		return s.schema.NumFields()
	}
	return 0
}

// Field returns the field at the given index
func (s *Schema) Field(index int) (arrow.Field, error) {
	if s.schema == nil {
		return arrow.Field{}, fmt.Errorf("schema is nil")
	}

	if index < 0 || index >= s.schema.NumFields() {
		return arrow.Field{}, fmt.Errorf("field index %d out of range", index)
	}

	return s.schema.Field(index), nil
}

// FieldByName returns the field with the given name
func (s *Schema) FieldByName(name string) (arrow.Field, error) {
	if s.schema == nil {
		return arrow.Field{}, fmt.Errorf("schema is nil")
	}

	for _, field := range s.schema.Fields() {
		if field.Name == name {
			return field, nil
		}
	}

	return arrow.Field{}, fmt.Errorf("field '%s' not found", name)
}

// HasField checks if a field with the given name exists
func (s *Schema) HasField(name string) bool {
	_, err := s.FieldByName(name)
	return err == nil
}

// String returns a string representation of the schema
func (s *Schema) String() string {
	if s.schema != nil {
		return s.schema.String()
	}
	return "nil schema"
}

// ToArrowSchema returns the underlying Arrow schema
func (s *Schema) ToArrowSchema() *arrow.Schema {
	return s.schema
}

// VectorField is a convenience function to create a vector field
func VectorField(name string, dimension int, dataType lancedb.VectorDataType, nullable bool) arrow.Field {
	var itemType arrow.DataType
	switch dataType {
	case lancedb.VectorDataTypeFloat16:
		itemType = arrow.PrimitiveTypes.Float32
	case lancedb.VectorDataTypeFloat32:
		itemType = arrow.PrimitiveTypes.Float32
	case lancedb.VectorDataTypeFloat64:
		itemType = arrow.PrimitiveTypes.Float64
	default:
		itemType = arrow.PrimitiveTypes.Float32
	}

	vectorType := arrow.FixedSizeListOf(int32(dimension), itemType)
	return arrow.Field{
		Name:     name,
		Type:     vectorType,
		Nullable: nullable,
	}
}
