// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#include "lancedb.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Compile-time check that *Table implements the optional
// schema-evolution capability extension.
var _ contracts.ITableSchemaEvolve = (*Table)(nil)

// AddColumns adds new columns to the table by evaluating SQL
// expressions over existing rows. Empty transforms slices, empty
// names, and empty expressions are rejected on the Go side before
// crossing the FFI to keep error messages local and predictable.
func (t *Table) AddColumns(_ context.Context, transforms []contracts.NewColumnTransform) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}
	if len(transforms) == 0 {
		return 0, fmt.Errorf("add_columns: transforms must be non-empty")
	}
	for i, tr := range transforms {
		if strings.TrimSpace(tr.Name) == "" {
			return 0, fmt.Errorf("add_columns: transforms[%d].Name is empty", i)
		}
		if strings.TrimSpace(tr.Expression) == "" {
			return 0, fmt.Errorf("add_columns: transforms[%d].Expression is empty", i)
		}
	}

	payload, err := json.Marshal(transforms)
	if err != nil {
		return 0, fmt.Errorf("add_columns: marshal transforms: %w", err)
	}
	cJSON := C.CString(string(payload))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cJSON))

	var version C.uint64_t
	result := C.simple_lancedb_table_add_columns(t.handle, cJSON, &version)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return 0, fmt.Errorf("failed to add columns: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return 0, fmt.Errorf("failed to add columns: unknown error")
	}
	return uint64(version), nil
}

// AlterColumns renames columns and/or toggles their nullability.
// Each entry must change at least one attribute — alterations with
// neither rename nor nullable set are rejected as caller bugs (the
// backend would otherwise produce a no-op commit).
func (t *Table) AlterColumns(_ context.Context, alterations []contracts.ColumnAlteration) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}
	if len(alterations) == 0 {
		return 0, fmt.Errorf("alter_columns: alterations must be non-empty")
	}
	for i, a := range alterations {
		if strings.TrimSpace(a.Path) == "" {
			return 0, fmt.Errorf("alter_columns: alterations[%d].Path is empty", i)
		}
		if a.Rename == nil && a.Nullable == nil {
			return 0, fmt.Errorf("alter_columns: alterations[%d] has no rename or nullable change", i)
		}
		if a.Rename != nil && strings.TrimSpace(*a.Rename) == "" {
			return 0, fmt.Errorf("alter_columns: alterations[%d].Rename is empty string", i)
		}
	}

	payload, err := json.Marshal(alterations)
	if err != nil {
		return 0, fmt.Errorf("alter_columns: marshal alterations: %w", err)
	}
	cJSON := C.CString(string(payload))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cJSON))

	var version C.uint64_t
	result := C.simple_lancedb_table_alter_columns(t.handle, cJSON, &version)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return 0, fmt.Errorf("failed to alter columns: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return 0, fmt.Errorf("failed to alter columns: unknown error")
	}
	return uint64(version), nil
}

// DropColumns removes the named columns from the table. The on-disk
// bytes are reclaimed on the next OptimizeCompact — DropColumns itself
// only updates the manifest.
func (t *Table) DropColumns(_ context.Context, names []string) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("drop_columns: names must be non-empty")
	}
	for i, n := range names {
		if strings.TrimSpace(n) == "" {
			return 0, fmt.Errorf("drop_columns: names[%d] is empty", i)
		}
	}

	payload, err := json.Marshal(names)
	if err != nil {
		return 0, fmt.Errorf("drop_columns: marshal names: %w", err)
	}
	cJSON := C.CString(string(payload))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cJSON))

	var version C.uint64_t
	result := C.simple_lancedb_table_drop_columns(t.handle, cJSON, &version)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return 0, fmt.Errorf("failed to drop columns: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return 0, fmt.Errorf("failed to drop columns: unknown error")
	}
	return uint64(version), nil
}
