// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"bytes"
	"fmt"
	"io"
	"math"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

// recordsToIPCBytes serializes one or more Arrow records into an Arrow IPC
// file-format buffer. It returns nil for an empty slice; callers that need a
// schema-only file use schemaToIPCBytes instead.
func recordsToIPCBytes(records []arrow.Record) ([]byte, error) {
	if len(records) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	writer, err := ipc.NewFileWriter(&seekBuffer{&buf}, ipc.WithSchema(records[0].Schema()))
	if err != nil {
		return nil, fmt.Errorf("failed to create IPC writer: %w", err)
	}
	for _, record := range records {
		if err := writer.Write(record); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to write record to IPC: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close IPC writer: %w", err)
	}
	return buf.Bytes(), nil
}

// schemaToIPCBytes writes a schema-only Arrow IPC file.
func schemaToIPCBytes(schema *arrow.Schema) ([]byte, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	var buf bytes.Buffer
	writer, err := ipc.NewFileWriter(&seekBuffer{&buf}, ipc.WithSchema(schema))
	if err != nil {
		return nil, fmt.Errorf("failed to create schema IPC writer: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close schema IPC writer: %w", err)
	}
	return buf.Bytes(), nil
}

// ipcBytesToRecords decodes every record batch in an Arrow IPC file.
// It returns an empty slice (not nil) for a schema-only file.
func ipcBytesToRecords(ipcBytes []byte) ([]arrow.Record, *arrow.Schema, error) {
	if len(ipcBytes) == 0 {
		return nil, nil, nil
	}
	reader, err := ipc.NewFileReader(bytes.NewReader(ipcBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create IPC reader: %w", err)
	}
	defer reader.Close()

	schema := reader.Schema()
	records := make([]arrow.Record, 0, reader.NumRecords())
	for i := 0; i < reader.NumRecords(); i++ {
		rec, err := reader.Record(i)
		if err != nil {
			for _, kept := range records {
				kept.Release()
			}
			return nil, nil, fmt.Errorf("failed to read IPC record batch %d: %w", i, err)
		}
		rec.Retain()
		records = append(records, rec)
	}
	return records, schema, nil
}

// ipcBytesToRecord decodes an Arrow IPC file into a single record. Every
// result batch contributes its rows; Arrow batches are concatenated by column
// so the context/single-record API never silently drops a later batch.
func ipcBytesToRecord(ipcBytes []byte) (arrow.Record, error) {
	records, schema, err := ipcBytesToRecords(ipcBytes)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return nil, nil
	}
	defer func() {
		for _, rec := range records {
			rec.Release()
		}
	}()

	if len(records) == 0 {
		pool := memory.NewGoAllocator()
		cols := make([]arrow.Array, schema.NumFields())
		for i, field := range schema.Fields() {
			builder := array.NewBuilder(pool, field.Type)
			cols[i] = builder.NewArray()
			builder.Release()
		}
		rec := array.NewRecord(schema, cols, 0)
		for _, col := range cols {
			col.Release()
		}
		return rec, nil
	}
	if len(records) == 1 {
		rec := records[0]
		rec.Retain()
		return rec, nil
	}

	columns := make([]arrow.Array, schema.NumFields())
	for columnIndex := range columns {
		arrays := make([]arrow.Array, len(records))
		for recordIndex, record := range records {
			arrays[recordIndex] = record.Column(columnIndex)
		}
		concatenated, err := array.Concatenate(arrays, memory.DefaultAllocator)
		if err != nil {
			for _, column := range columns[:columnIndex] {
				column.Release()
			}
			return nil, fmt.Errorf("concatenate column %s across %d batches: %w", schema.Field(columnIndex).Name, len(records), err)
		}
		columns[columnIndex] = concatenated
	}
	totalRows := int64(0)
	for _, record := range records {
		totalRows += record.NumRows()
	}
	record := array.NewRecord(schema, columns, totalRows)
	for _, column := range columns {
		column.Release()
	}
	return record, nil
}

// ipcBytesToMaps converts Arrow rows to the legacy map representation. Numbers
// are float64 because v0.1.2 decoded a JSON response; keeping that shape avoids
// a silent type change for existing callers. The context/single-record API
// returns typed Arrow records for callers that need exact widths.
func ipcBytesToMaps(ipcBytes []byte) ([]map[string]interface{}, error) {
	return ipcBytesToMapsWith(ipcBytes, arrowValueToJSONLegacy)
}

func ipcBytesToMapsV2(ipcBytes []byte) ([]map[string]interface{}, error) {
	return ipcBytesToMapsWith(ipcBytes, arrowValueToJSON)
}

func ipcBytesToMapsWith(ipcBytes []byte, convert func(arrow.Array, int) (interface{}, error)) ([]map[string]interface{}, error) {
	records, _, err := ipcBytesToRecords(ipcBytes)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, rec := range records {
			rec.Release()
		}
	}()
	if len(records) == 0 {
		return []map[string]interface{}{}, nil
	}

	rows := make([]map[string]interface{}, 0)
	for _, rec := range records {
		schema := rec.Schema()
		for row := 0; row < int(rec.NumRows()); row++ {
			values := make(map[string]interface{}, schema.NumFields())
			for col := 0; col < schema.NumFields(); col++ {
				value, err := convert(rec.Column(col), row)
				if err != nil {
					return nil, fmt.Errorf("convert column %s: %w", schema.Field(col).Name, err)
				}
				values[schema.Field(col).Name] = value
			}
			rows = append(rows, values)
		}
	}
	return rows, nil
}

// arrowValueToJSONLegacy reproduces the v0.1.2 conversion switch. In
// particular, UInt8 and variable-length List are unsupported placeholders, and
// only FixedSizeList is recursively decoded.
func arrowValueToJSONLegacy(column arrow.Array, index int) (interface{}, error) {
	if column.IsNull(index) {
		return nil, nil
	}
	switch values := column.(type) {
	case *array.Int32:
		return float64(values.Value(index)), nil
	case *array.Int64:
		return float64(values.Value(index)), nil
	case *array.Float32:
		value := float64(values.Value(index))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil
		}
		return value, nil
	case *array.Float64:
		value := values.Value(index)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil
		}
		return value, nil
	case *array.Boolean:
		return values.Value(index), nil
	case *array.String:
		return values.Value(index), nil
	case *array.FixedSizeList:
		start, end := values.ValueOffsets(index)
		child := values.ListValues()
		out := make([]interface{}, 0, end-start)
		for i := start; i < end; i++ {
			value, err := arrowValueToJSONLegacy(child, int(i))
			if err != nil {
				value = nil
			}
			out = append(out, value)
		}
		return out, nil
	default:
		return "Unsupported type: " + rustDebugType(column.DataType()), nil
	}
}

func arrowValueToJSON(column arrow.Array, index int) (interface{}, error) {
	if column.IsNull(index) {
		return nil, nil
	}
	switch values := column.(type) {
	case *array.Int32:
		return float64(values.Value(index)), nil
	case *array.Int64:
		return float64(values.Value(index)), nil
	case *array.Uint8:
		return float64(values.Value(index)), nil
	case *array.Uint16:
		return float64(values.Value(index)), nil
	case *array.Uint32:
		return float64(values.Value(index)), nil
	case *array.Uint64:
		return float64(values.Value(index)), nil
	case *array.Float32:
		value := float64(values.Value(index))
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil
		}
		return value, nil
	case *array.Float64:
		value := values.Value(index)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil
		}
		return value, nil
	case *array.Boolean:
		return values.Value(index), nil
	case *array.String:
		return values.Value(index), nil
	case *array.FixedSizeList:
		start, end := values.ValueOffsets(index)
		return listValuesToJSON(values.ListValues(), start, end)
	case *array.List:
		start, end := values.ValueOffsets(index)
		return listValuesToJSON(values.ListValues(), start, end)
	default:
		// The published Select path decoded a JSON response. Match the old
		// converter's Rust Debug placeholder for types it did not support,
		// rather than exposing a new Go-formatted or numeric value.
		return "Unsupported type: " + rustDebugType(column.DataType()), nil
	}
}

func rustDebugType(dataType arrow.DataType) string {
	switch value := dataType.(type) {
	case *arrow.Int8Type:
		return "Int8"
	case *arrow.Int16Type:
		return "Int16"
	case *arrow.Int32Type:
		return "Int32"
	case *arrow.Int64Type:
		return "Int64"
	case *arrow.Uint8Type:
		return "UInt8"
	case *arrow.Uint16Type:
		return "UInt16"
	case *arrow.Uint32Type:
		return "UInt32"
	case *arrow.Uint64Type:
		return "UInt64"
	case *arrow.Float16Type:
		return "Float16"
	case *arrow.Float32Type:
		return "Float32"
	case *arrow.Float64Type:
		return "Float64"
	case *arrow.BooleanType:
		return "Boolean"
	case *arrow.StringType:
		return "Utf8"
	case *arrow.BinaryType:
		return "Binary"
	case *arrow.LargeBinaryType:
		return "LargeBinary"
	case *arrow.FixedSizeBinaryType:
		return fmt.Sprintf("FixedSizeBinary(%d)", value.ByteWidth)
	case *arrow.BinaryViewType:
		return "BinaryView"
	case *arrow.StringViewType:
		return "Utf8View"
	case *arrow.LargeStringType:
		return "LargeUtf8"
	case *arrow.Date32Type:
		return "Date32"
	case *arrow.Date64Type:
		return "Date64"
	case *arrow.Time32Type:
		return fmt.Sprintf("Time32(%s)", rustTimeUnit(value.Unit))
	case *arrow.Time64Type:
		return fmt.Sprintf("Time64(%s)", rustTimeUnit(value.Unit))
	case *arrow.TimestampType:
		zone := "None"
		if value.TimeZone != "" {
			zone = fmt.Sprintf("Some(%q)", value.TimeZone)
		}
		return fmt.Sprintf("Timestamp(%s, %s)", rustTimeUnit(value.Unit), zone)
	case *arrow.DurationType:
		return fmt.Sprintf("Duration(%s)", rustTimeUnit(value.Unit))
	case *arrow.Decimal128Type:
		return fmt.Sprintf("Decimal128(%d, %d)", value.Precision, value.Scale)
	case *arrow.Decimal256Type:
		return fmt.Sprintf("Decimal256(%d, %d)", value.Precision, value.Scale)
	case *arrow.ListType:
		return "List(" + rustFieldDebug(value.ElemField()) + ")"
	case *arrow.FixedSizeListType:
		return fmt.Sprintf("FixedSizeList(%s, %d)", rustFieldDebug(value.ElemField()), value.Len())
	case *arrow.LargeListType:
		return "LargeList(" + rustFieldDebug(value.ElemField()) + ")"
	case *arrow.MapType:
		return "Map"
	case *arrow.StructType:
		return "Struct"
	default:
		return dataType.String()
	}
}

func rustFieldDebug(field arrow.Field) string {
	return fmt.Sprintf("Field { name: %q, data_type: %s, nullable: %t, dict_id: 0, dict_is_ordered: false, metadata: {} }", field.Name, rustDebugType(field.Type), field.Nullable)
}

func rustTimeUnit(unit arrow.TimeUnit) string {
	switch unit {
	case arrow.Second:
		return "Second"
	case arrow.Millisecond:
		return "Millisecond"
	case arrow.Microsecond:
		return "Microsecond"
	case arrow.Nanosecond:
		return "Nanosecond"
	default:
		return "Unknown"
	}
}

func listValuesToJSON(values arrow.Array, start, end int64) ([]interface{}, error) {
	out := make([]interface{}, 0, end-start)
	for i := start; i < end; i++ {
		value, err := arrowValueToJSON(values, int(i))
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

// seekBuffer lets the Arrow IPC file writer use an in-memory buffer.
type seekBuffer struct {
	*bytes.Buffer
}

func (sb *seekBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekEnd:
		return int64(sb.Len()), nil
	case io.SeekStart:
		if offset == 0 {
			sb.Reset()
			return 0, nil
		}
		return 0, fmt.Errorf("seeking to non-zero position not supported")
	case io.SeekCurrent:
		return int64(sb.Len()), nil
	default:
		return 0, fmt.Errorf("unsupported whence value")
	}
}
