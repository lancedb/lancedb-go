package lancedb

//go:generate go run ../../cmd/download-binaries

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
	"runtime"
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
)

// Connect establishes a connection to a LanceDB database with context
//
//nolint:gocritic
func Connect(_ context.Context, uri string, options *contracts.ConnectionOptions) (contracts.IConnection, error) {
	// Initialize the library
	C.simple_lancedb_init()

	cURI := C.CString(uri)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cURI))

	// #nosec G103 - FFI handle for C interop with Rust library
	var handle unsafe.Pointer
	var result *C.SimpleResult

	// Use storage options if provided
	if options != nil && options.StorageOptions != nil {
		// Serialize storage options to JSON
		optionsJSON, err := json.Marshal(options.StorageOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize storage options: %w", err)
		}

		cOptions := C.CString(string(optionsJSON))
		// #nosec G103 - Required for freeing C allocated string memory
		defer C.free(unsafe.Pointer(cOptions))

		result = C.simple_lancedb_connect_with_options(cURI, cOptions, &handle)
	} else {
		// Use basic connection without storage options
		result = C.simple_lancedb_connect(cURI, &handle)
	}

	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			errorMsg := C.GoString(result.ERROR_MESSAGE)
			return nil, fmt.Errorf("failed to connect to LanceDB at %s: %s", uri, errorMsg)
		}
		return nil, fmt.Errorf("failed to connect to LanceDB at %s: unknown error", uri)
	}

	conn := internal.NewConnection(handle, false)

	// Set finalizer to ensure cleanup
	runtime.SetFinalizer(conn, contracts.IConnection.Close)

	return conn, nil
}
