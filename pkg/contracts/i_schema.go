package contracts

import "github.com/apache/arrow/go/v17/arrow"

type ISchema interface {
	Fields() []arrow.Field
	NumFields() int
	Field(index int) (arrow.Field, error)
	FieldByName(name string) (arrow.Field, error)
	HasField(name string) bool
	String() string
	ToArrowSchema() *arrow.Schema
}

type ISchemaBuilder interface {
	AddField(name string, dataType arrow.DataType, nullable bool) ISchemaBuilder
	AddVectorField(name string, dimension int, dataType VectorDataType, nullable bool) ISchemaBuilder
	AddInt32Field(name string, nullable bool) ISchemaBuilder
	AddInt64Field(name string, nullable bool) ISchemaBuilder
	AddFloat32Field(name string, nullable bool) ISchemaBuilder
	AddFloat64Field(name string, nullable bool) ISchemaBuilder
	AddStringField(name string, nullable bool) ISchemaBuilder
	AddBinaryField(name string, nullable bool) ISchemaBuilder
	AddBooleanField(name string, nullable bool) ISchemaBuilder
	AddTimestampField(name string, unit arrow.TimeUnit, nullable bool) ISchemaBuilder
	Build() (ISchema, error)
}

// VectorDataType represents the data type for vector fields
type VectorDataType int

const (
	VectorDataTypeFloat16 VectorDataType = iota
	VectorDataTypeFloat32
	VectorDataTypeFloat64
)
