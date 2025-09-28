// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../../lib/darwin_amd64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../../lib/darwin_arm64/liblancedb_go.a -framework Security -framework CoreFoundation
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../../lib/linux_amd64/liblancedb_go.a -lm -ldl -lpthread
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/../../lib/linux_arm64/liblancedb_go.a -lm -ldl -lpthread
#cgo windows,amd64 LDFLAGS: ${SRCDIR}/../../lib/windows_amd64/liblancedb_go.a
#include "lancedb.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/ipc"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Connection represents a connection to a LanceDB database
type Connection struct {
	// #nosec G103 - FFI handle for C interop with Rust library
	handle unsafe.Pointer
	mu     sync.RWMutex
	closed bool
}

// #nosec G103 - Function parameter for FFI handle from C interop
func NewConnection(handle unsafe.Pointer, closed bool) *Connection {
	return &Connection{
		handle: handle,
		closed: closed,
	}
}

var _ contracts.IConnection = (*Connection)(nil)

// Close closes the connection to the database
//
//nolint:gocritic
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed || c.handle == nil {
		return nil
	}

	result := C.simple_lancedb_close(c.handle)
	defer C.simple_lancedb_result_free(result)

	c.handle = nil
	c.closed = true
	runtime.SetFinalizer(c, nil)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to close connection: %s", errorMsg)
		}
		return fmt.Errorf("failed to close connection: unknown error")
	}

	return nil
}

func (c *Connection) IsClosed() bool {
	return c.closed
}

// TableNames returns a list of table names in the database with context
//
//nolint:gocritic
func (c *Connection) TableNames(_ context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.handle == nil {
		return nil, fmt.Errorf("connection is closed")
	}

	var cNames **C.char
	var count C.int

	result := C.simple_lancedb_table_names(c.handle, &cNames, &count)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to get table names: %s", errorMsg)
		}
		return nil, fmt.Errorf("failed to get table names: unknown error")
	}

	if count == 0 {
		return []string{}, nil
	}

	// Convert C string array to Go string slice
	tableNames := make([]string, int(count))
	// #nosec G103 - Safe conversion of C array pointer to Go slice with known bounds
	cNamesSlice := (*[1 << 20]*C.char)(unsafe.Pointer(cNames))[:count:count]
	for i, cName := range cNamesSlice {
		tableNames[i] = C.GoString(cName)
	}

	// Free the C memory
	C.simple_lancedb_free_table_names(cNames, count)

	return tableNames, nil
}

// OpenTable opens an existing table in the database with context
//
//nolint:gocritic
func (c *Connection) OpenTable(_ context.Context, name string) (contracts.ITable, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.handle == nil {
		return nil, fmt.Errorf("connection is closed")
	}

	cName := C.CString(name)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cName))

	// #nosec G103 - FFI handle for table from C interop
	var tableHandle unsafe.Pointer
	result := C.simple_lancedb_open_table(c.handle, cName, &tableHandle)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to open table %s: %s", name, errorMsg)
		}
		return nil, fmt.Errorf("failed to open table %s: unknown error", name)
	}

	table := &Table{
		name:       name,
		connection: c,
		handle:     tableHandle,
		closed:     false,
	}

	// Set finalizer to ensure cleanup
	runtime.SetFinalizer(table, (*Table).Close)

	return table, nil
}

// CreateTable creates a new table in the database with context
func (c *Connection) CreateTable(ctx context.Context, name string, schema contracts.ISchema) (contracts.ITable, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.handle == nil {
		return nil, fmt.Errorf("connection is closed")
	}

	// Convert schema to Arrow IPC bytes (more efficient than JSON)
	schemaIPC, err := c.schemaToIPC(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to IPC: %w", err)
	}

	cName := C.CString(name)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cName))

	// Convert Go bytes to C pointers
	var cSchemaPtr *C.uchar
	if len(schemaIPC) > 0 {
		// #nosec G103 - Safe conversion of Go slice to C pointer for FFI
		cSchemaPtr = (*C.uchar)(unsafe.Pointer(&schemaIPC[0]))
	}

	result := C.simple_lancedb_create_table_with_ipc(
		c.handle,
		cName,
		cSchemaPtr,
		C.size_t(len(schemaIPC)),
	)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to create table %s: %s", name, errorMsg)
		}
		return nil, fmt.Errorf("failed to create table %s: unknown error", name)
	}

	// After successful creation, open the table to get a handle
	return c.OpenTable(ctx, name)
}

// DropTable drops a table from the database with context
func (c *Connection) DropTable(_ context.Context, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.handle == nil {
		return fmt.Errorf("connection is closed")
	}

	cName := C.CString(name)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cName))

	result := C.simple_lancedb_drop_table(c.handle, cName)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return fmt.Errorf("failed to drop table %s: %s", name, errorMsg)
		}
		return fmt.Errorf("failed to drop table %s: unknown error", name)
	}

	return nil
}

// schemaToIPC converts a Schema to Arrow IPC bytes for efficient transfer
//
//nolint:wrapcheck
func (c *Connection) schemaToIPC(schema contracts.ISchema) ([]byte, error) {
	if schema.ToArrowSchema() == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	// Create a temporary in-memory file using os.CreateTemp with os.DevNull pattern
	// This will work with the FileWriter which requires WriteSeeker interface
	tmpFile, err := os.CreateTemp("", "schema_*.arrow")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	// Use FileWriter to create a file format that can be read by FileReader in Rust
	writer, err := ipc.NewFileWriter(tmpFile, ipc.WithSchema(schema.ToArrowSchema()))
	if err != nil {
		return nil, fmt.Errorf("failed to create IPC file writer: %w", err)
	}

	// Close the writer to finalize the file format (no data, just schema)
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close IPC writer: %w", err)
	}

	// Seek back to beginning and read the file contents
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to beginning: %w", err)
	}

	return io.ReadAll(tmpFile)
}

// schemaToJSON converts a Schema to JSON string for Rust bindings (deprecated in favor of schemaToIPC)
func (c *Connection) schemaToJSON(schema Schema) (string, error) {
	if schema.schema == nil {
		return "", fmt.Errorf("schema is nil")
	}

	fields := make([]map[string]interface{}, 0, schema.NumFields())

	for i := 0; i < schema.NumFields(); i++ {
		field, err := schema.Field(i)
		if err != nil {
			return "", fmt.Errorf("failed to get field %d: %w", i, err)
		}

		fieldMap := map[string]interface{}{
			"name":     field.Name,
			"nullable": field.Nullable,
		}

		// Convert Arrow DataType to string representation
		typeStr, err := c.arrowTypeToString(field.Type)
		if err != nil {
			return "", fmt.Errorf("failed to convert type for field %s: %w", field.Name, err)
		}
		fieldMap["type"] = typeStr

		fields = append(fields, fieldMap)
	}

	schemaMap := map[string]interface{}{
		"fields": fields,
	}

	jsonBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// //nolint:gocyclo
// arrowTypeToString converts an Arrow DataType to string representation
func (c *Connection) arrowTypeToString(dataType arrow.DataType) (string, error) {
	switch dt := dataType.(type) {
	case *arrow.Int8Type:
		return "int8", nil
	case *arrow.Int16Type:
		return "int16", nil
	case *arrow.Int32Type:
		return "int32", nil
	case *arrow.Int64Type:
		return "int64", nil
	case *arrow.Float16Type:
		return "float16", nil
	case *arrow.Float32Type:
		return "float32", nil
	case *arrow.Float64Type:
		return "float64", nil
	case *arrow.StringType:
		return "string", nil
	case *arrow.BinaryType:
		return "binary", nil
	case *arrow.BooleanType:
		return "boolean", nil
	case *arrow.FixedSizeListType:
		// Handle vector types (fixed size list of floats)
		if dt.Elem().ID() == arrow.FLOAT16 {
			return fmt.Sprintf("fixed_size_list[float16;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.FLOAT32 {
			return fmt.Sprintf("fixed_size_list[float32;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.FLOAT64 {
			return fmt.Sprintf("fixed_size_list[float64;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.INT8 {
			return fmt.Sprintf("fixed_size_list[int8;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.INT16 {
			return fmt.Sprintf("fixed_size_list[int16;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.INT32 {
			return fmt.Sprintf("fixed_size_list[int32;%d]", dt.Len()), nil
		}
		if dt.Elem().ID() == arrow.INT64 {
			return fmt.Sprintf("fixed_size_list[int64;%d]", dt.Len()), nil
		}

		return "", fmt.Errorf("unsupported fixed size list element type: %v", dt.Elem())
	default:
		return "", fmt.Errorf("unsupported Arrow type: %v", dataType)
	}
}
