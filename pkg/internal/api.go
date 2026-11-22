// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func loadAPI() (*native.API, error) {
	api, err := native.Load()
	if err != nil {
		return nil, fmt.Errorf("load lancedb-c native library: %w", err)
	}
	return api, nil
}

func goString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

func takeError(api *native.API, p *byte) string {
	if p == nil {
		return ""
	}
	message := goString(p)
	api.FreeString(p)
	return message
}

func check(api *native.API, code int32, errorMessage *byte, operation string) error {
	if code == 0 {
		// The C API clears the slot on entry and sets it only on failure.
		// Free defensively if a future implementation violates that.
		if errorMessage != nil {
			api.FreeString(errorMessage)
		}
		return nil
	}
	message := takeError(api, errorMessage)
	if message == "" {
		message = fmt.Sprintf("lancedb-c error %d", code)
	}
	return fmt.Errorf("%s: %s", operation, message)
}

func cString(value string) []byte {
	return append([]byte(value), 0)
}

func cStringPtr(value string) (*byte, []byte) {
	data := cString(value)
	return &data[0], data
}

// cStringArray returns the pointer array and a keepalive closure. Call the
// closure after the C call returns; the C API borrows these strings only for
// the duration of the call.
func cStringArray(values []string) (**byte, func()) {
	if len(values) == 0 {
		return nil, func() {}
	}
	data := make([][]byte, len(values))
	ptrs := make([]*byte, len(values))
	for i, value := range values {
		data[i] = cString(value)
		ptrs[i] = &data[i][0]
	}
	return (**byte)(unsafe.Pointer(&ptrs[0])), func() {
		runtime.KeepAlive(data)
		runtime.KeepAlive(ptrs)
	}
}

func goStringArray(ptrs **byte, count uintptr) []string {
	if ptrs == nil || count == 0 {
		return nil
	}
	values := unsafe.Slice(ptrs, int(count))
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = goString(value)
	}
	return out
}

// newIPCReader imports an Arrow IPC file into a native record-batch reader.
// The returned schema slot is optional; when requested, the caller must free it
// with lancedb_free_arrow_schema after lancedb_table_create returns.
func newIPCReader(api *native.API, ipc []byte, wantSchema bool) (reader, schema uintptr, err error) {
	var errorMessage *byte
	var data *byte
	if len(ipc) > 0 {
		data = &ipc[0]
	}
	code := api.RecordBatchReaderFromIPCFile(data, uintptr(len(ipc)), &reader, &schema, &errorMessage)
	runtime.KeepAlive(ipc)
	if err := check(api, code, errorMessage, "read Arrow IPC file"); err != nil {
		if reader != 0 {
			api.RecordBatchReaderFree(reader)
		}
		if schema != 0 {
			api.FreeArrowSchema(schema)
		}
		return 0, 0, err
	}
	if !wantSchema && schema != 0 {
		api.FreeArrowSchema(schema)
		schema = 0
	}
	return reader, schema, nil
}

func queryResultIPC(api *native.API, result uintptr) ([]byte, error) {
	var buffer native.IpcBuffer
	var errorMessage *byte
	code := api.QueryResultToIPCFile(result, &buffer, &errorMessage)
	if err := check(api, code, errorMessage, "encode query result as Arrow IPC"); err != nil {
		if buffer.Data != nil {
			api.IpcBufferFree(&buffer)
		}
		return nil, err
	}
	if buffer.Data == nil || buffer.Len == 0 {
		api.IpcBufferFree(&buffer)
		return nil, nil
	}
	out := make([]byte, int(buffer.Len))
	copy(out, unsafe.Slice(buffer.Data, int(buffer.Len)))
	api.IpcBufferFree(&buffer)
	return out, nil
}

func tableSchemaIPC(api *native.API, table uintptr) ([]byte, error) {
	var buffer native.IpcBuffer
	var errorMessage *byte
	code := api.TableSchemaIPCFile(table, &buffer, &errorMessage)
	if err := check(api, code, errorMessage, "encode table schema as Arrow IPC"); err != nil {
		if buffer.Data != nil {
			api.IpcBufferFree(&buffer)
		}
		return nil, err
	}
	if buffer.Data == nil || buffer.Len == 0 {
		api.IpcBufferFree(&buffer)
		return nil, fmt.Errorf("lancedb-c returned an empty table schema")
	}
	out := make([]byte, int(buffer.Len))
	copy(out, unsafe.Slice(buffer.Data, int(buffer.Len)))
	api.IpcBufferFree(&buffer)
	return out, nil
}
