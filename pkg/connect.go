package lancedb

/*
#cgo CFLAGS: -I${SRCDIR}/../rust/target/generated/include
#cgo darwin LDFLAGS: -L${SRCDIR}/../rust/target/generated/lib -llancedb_go -framework Security -framework CoreFoundation
#cgo linux LDFLAGS: -L${SRCDIR}/../rust/target/generated/lib -llancedb_go
#include "lancedb.h"
*/
import "C"

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
	defer C.free(unsafe.Pointer(cURI))

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
