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
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// Compile-time check that *Table implements the optional time-travel
// capability extension.
var _ contracts.ITableTimeTravel = (*Table)(nil)

// ListVersions returns the dataset's version history. Order matches
// the backend's response.
func (t *Table) ListVersions(_ context.Context) ([]contracts.VersionInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	var versionsJSON *C.char
	result := C.simple_lancedb_table_list_versions(t.handle, &versionsJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return nil, fmt.Errorf("failed to list versions: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return nil, fmt.Errorf("failed to list versions: unknown error")
	}

	if versionsJSON == nil {
		return []contracts.VersionInfo{}, nil
	}
	jsonStr := C.GoString(versionsJSON)
	C.simple_lancedb_free_string(versionsJSON)

	var versions []contracts.VersionInfo
	if err := json.Unmarshal([]byte(jsonStr), &versions); err != nil {
		return nil, fmt.Errorf("list_versions: failed to parse result JSON: %w", err)
	}
	return versions, nil
}

// Checkout pins the table to a specific version. Subsequent reads
// see that snapshot. Writes are rejected until the pin is dropped
// with CheckoutLatest or promoted with Restore.
func (t *Table) Checkout(_ context.Context, version uint64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	result := C.simple_lancedb_table_checkout(t.handle, C.uint64_t(version))
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to checkout version %d: %s", version, C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to checkout version %d: unknown error", version)
	}
	return nil
}

// CheckoutTag pins the table to the version referenced by the given
// tag.
func (t *Table) CheckoutTag(_ context.Context, tag string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	cTag := C.CString(tag)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cTag))

	result := C.simple_lancedb_table_checkout_tag(t.handle, cTag)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to checkout tag %q: %s", tag, C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to checkout tag %q: unknown error", tag)
	}
	return nil
}

// CheckoutLatest drops any prior checkout pin and resumes tracking
// the latest manifest.
func (t *Table) CheckoutLatest(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	result := C.simple_lancedb_table_checkout_latest(t.handle)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to checkout latest: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to checkout latest: unknown error")
	}
	return nil
}

// Restore promotes the currently checked-out version to a new latest
// manifest. Errors when the table is not in a checked-out state.
func (t *Table) Restore(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}

	result := C.simple_lancedb_table_restore(t.handle)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to restore: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to restore: unknown error")
	}
	return nil
}

// TagList returns every tag on the table, keyed by tag name.
func (t *Table) TagList(_ context.Context) (map[string]contracts.TagInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	var tagsJSON *C.char
	result := C.simple_lancedb_table_tags_list(t.handle, &tagsJSON)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return nil, fmt.Errorf("failed to list tags: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return nil, fmt.Errorf("failed to list tags: unknown error")
	}

	if tagsJSON == nil {
		return map[string]contracts.TagInfo{}, nil
	}
	jsonStr := C.GoString(tagsJSON)
	C.simple_lancedb_free_string(tagsJSON)

	var tags map[string]contracts.TagInfo
	if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
		return nil, fmt.Errorf("tags_list: failed to parse result JSON: %w", err)
	}
	return tags, nil
}

// TagGetVersion resolves a tag to its pinned version. Errors when the
// tag does not exist.
func (t *Table) TagGetVersion(_ context.Context, tag string) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return 0, fmt.Errorf("table is closed")
	}
	if tag == "" {
		return 0, fmt.Errorf("tag name cannot be empty")
	}

	cTag := C.CString(tag)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cTag))

	var version C.uint64_t
	result := C.simple_lancedb_table_tags_get_version(t.handle, cTag, &version)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return 0, fmt.Errorf("failed to resolve tag %q: %s", tag, C.GoString(result.ERROR_MESSAGE))
		}
		return 0, fmt.Errorf("failed to resolve tag %q: unknown error", tag)
	}
	return uint64(version), nil
}

// TagCreate creates a new tag pointing at the given version. Errors
// when the tag already exists or the version is unknown.
func (t *Table) TagCreate(_ context.Context, tag string, version uint64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	cTag := C.CString(tag)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cTag))

	result := C.simple_lancedb_table_tags_create(t.handle, cTag, C.uint64_t(version))
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to create tag %q: %s", tag, C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to create tag %q: unknown error", tag)
	}
	return nil
}

// TagDelete deletes a tag. Errors when the tag does not exist.
func (t *Table) TagDelete(_ context.Context, tag string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	cTag := C.CString(tag)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cTag))

	result := C.simple_lancedb_table_tags_delete(t.handle, cTag)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to delete tag %q: %s", tag, C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to delete tag %q: unknown error", tag)
	}
	return nil
}

// TagUpdate moves an existing tag to a new version. Errors when the
// tag does not exist or the version is unknown.
func (t *Table) TagUpdate(_ context.Context, tag string, version uint64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return fmt.Errorf("table is closed")
	}
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	cTag := C.CString(tag)
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cTag))

	result := C.simple_lancedb_table_tags_update(t.handle, cTag, C.uint64_t(version))
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return fmt.Errorf("failed to update tag %q: %s", tag, C.GoString(result.ERROR_MESSAGE))
		}
		return fmt.Errorf("failed to update tag %q: unknown error", tag)
	}
	return nil
}
