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
	"time"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

// MergeInsertBuilder implements contracts.IMergeInsertBuilder.
type MergeInsertBuilder struct {
	table *Table

	on []string

	whenMatchedUpdateAll bool
	whenMatchedCondition *string
	whenNotMatchedInsert bool
	whenNotMatchedDelete bool
	whenNotMatchedFilter *string
	timeoutMillis        *uint64
	useIndex             *bool
}

var _ contracts.IMergeInsertBuilder = (*MergeInsertBuilder)(nil)

func (b *MergeInsertBuilder) WhenMatchedUpdateAll(condition *string) contracts.IMergeInsertBuilder {
	b.whenMatchedUpdateAll = true
	b.whenMatchedCondition = condition
	return b
}

func (b *MergeInsertBuilder) WhenNotMatchedInsertAll() contracts.IMergeInsertBuilder {
	b.whenNotMatchedInsert = true
	return b
}

func (b *MergeInsertBuilder) WhenNotMatchedBySourceDelete(filter *string) contracts.IMergeInsertBuilder {
	b.whenNotMatchedDelete = true
	b.whenNotMatchedFilter = filter
	return b
}

func (b *MergeInsertBuilder) Timeout(d time.Duration) contracts.IMergeInsertBuilder {
	if d < 0 {
		d = 0
	}
	ms := uint64(d / time.Millisecond)
	b.timeoutMillis = &ms
	return b
}

func (b *MergeInsertBuilder) UseIndex(useIndex bool) contracts.IMergeInsertBuilder {
	b.useIndex = &useIndex
	return b
}

type mergeInsertConfig struct {
	On                           []string `json:"on"`
	WhenMatchedUpdateAll         bool     `json:"when_matched_update_all"`
	WhenMatchedCondition         *string  `json:"when_matched_condition"`
	WhenNotMatchedInsertAll      bool     `json:"when_not_matched_insert_all"`
	WhenNotMatchedBySourceDelete bool     `json:"when_not_matched_by_source_delete"`
	WhenNotMatchedBySourceFilter *string  `json:"when_not_matched_by_source_filter"`
	TimeoutMs                    *uint64  `json:"timeout_ms"`
	UseIndex                     *bool    `json:"use_index"`
}

func (b *MergeInsertBuilder) Execute(_ context.Context, records []arrow.Record) (*contracts.MergeResult, error) {
	if len(b.on) == 0 {
		return nil, fmt.Errorf("merge_insert: 'on' must contain at least one column")
	}
	if !b.whenMatchedUpdateAll && !b.whenNotMatchedInsert && !b.whenNotMatchedDelete {
		return nil, fmt.Errorf("merge_insert: no merge actions configured; call at least one of " +
			"WhenMatchedUpdateAll, WhenNotMatchedInsertAll, WhenNotMatchedBySourceDelete")
	}

	t := b.table
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || t.handle == nil {
		return nil, fmt.Errorf("table is closed")
	}

	// Empty-records case: pass a zero-length IPC payload and let the Rust side
	// fall back to the table's own schema. Calling t.Schema() here would
	// re-acquire t.mu.RLock while we already hold it — a pending Close() writer
	// would deadlock the two RLock acquisitions.
	ipcBytes, err := recordsToIPCBytes(records)
	if err != nil {
		return nil, fmt.Errorf("merge_insert: %w", err)
	}

	cfg := mergeInsertConfig{
		On:                           b.on,
		WhenMatchedUpdateAll:         b.whenMatchedUpdateAll,
		WhenMatchedCondition:         b.whenMatchedCondition,
		WhenNotMatchedInsertAll:      b.whenNotMatchedInsert,
		WhenNotMatchedBySourceDelete: b.whenNotMatchedDelete,
		WhenNotMatchedBySourceFilter: b.whenNotMatchedFilter,
		TimeoutMs:                    b.timeoutMillis,
		UseIndex:                     b.useIndex,
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("merge_insert: failed to marshal config: %w", err)
	}

	cConfig := C.CString(string(cfgBytes))
	// #nosec G103 - Required for freeing C allocated string memory
	defer C.free(unsafe.Pointer(cConfig))

	var ipcPtr *C.uchar
	if len(ipcBytes) > 0 {
		// #nosec G103 - Safe conversion of Go slice to C array pointer for FFI
		ipcPtr = (*C.uchar)(unsafe.Pointer(&ipcBytes[0]))
	}

	var resultJSON *C.char
	result := C.simple_lancedb_table_merge_insert_ipc(
		t.handle,
		cConfig,
		ipcPtr,
		C.size_t(uintptr(len(ipcBytes))),
		&resultJSON,
	)
	defer C.simple_lancedb_result_free(result)

	if !result.SUCCESS {
		if result.ERROR_MESSAGE != nil {
			return nil, fmt.Errorf("merge_insert failed: %s", C.GoString(result.ERROR_MESSAGE))
		}
		return nil, fmt.Errorf("merge_insert failed: unknown error")
	}

	if resultJSON == nil {
		return &contracts.MergeResult{}, nil
	}
	jsonStr := C.GoString(resultJSON)
	C.simple_lancedb_free_string(resultJSON)

	var mr contracts.MergeResult
	if err := json.Unmarshal([]byte(jsonStr), &mr); err != nil {
		return nil, fmt.Errorf("merge_insert: failed to parse result JSON: %w", err)
	}
	return &mr, nil
}
