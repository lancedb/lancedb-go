// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/apache/arrow/go/v17/arrow"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

// MergeInsertBuilder implements contracts.IMergeInsertBuilder.
type MergeInsertBuilder struct {
	table *Table
	on    []string

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

func (b *MergeInsertBuilder) Execute(_ context.Context, records []arrow.Record) (*contracts.MergeResult, error) {
	if len(b.on) == 0 {
		return nil, fmt.Errorf("merge_insert: 'on' must contain at least one column")
	}
	if !b.whenMatchedUpdateAll && !b.whenNotMatchedInsert && !b.whenNotMatchedDelete {
		return nil, fmt.Errorf("merge_insert: no merge actions configured; call at least one of WhenMatchedUpdateAll, WhenNotMatchedInsertAll, WhenNotMatchedBySourceDelete")
	}

	t := b.table
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}

	var ipcBytes []byte
	if len(records) == 0 {
		// The SDK uses the reader's schema as the empty source schema; read the
		// table schema through the native API while holding the Go read lock.
		tableSchemaIPC, err := tableSchemaIPC(api, t.handle)
		if err != nil {
			return nil, fmt.Errorf("merge_insert: %w", err)
		}
		_, schema, err := ipcBytesToRecords(tableSchemaIPC)
		if err != nil {
			return nil, fmt.Errorf("merge_insert: %w", err)
		}
		if schema == nil {
			return nil, fmt.Errorf("merge_insert: table has no schema")
		}
		ipcBytes, err = schemaToIPCBytes(schema)
		if err != nil {
			return nil, fmt.Errorf("merge_insert: %w", err)
		}
	} else {
		ipcBytes, err = recordsToIPCBytes(records)
		if err != nil {
			return nil, fmt.Errorf("merge_insert: %w", err)
		}
	}
	reader, _, err := newIPCReader(api, ipcBytes, false)
	if err != nil {
		return nil, fmt.Errorf("merge_insert: %w", err)
	}

	options := native.MergeInsertOptions{
		WhenMatchedUpdateAll:         boolInt(b.whenMatchedUpdateAll),
		WhenNotMatchedInsertAll:      boolInt(b.whenNotMatchedInsert),
		WhenNotMatchedBySourceDelete: boolInt(b.whenNotMatchedDelete),
		UseIndex:                     -1,
		TimeoutMS:                    -1,
	}
	var conditionData, filterData []byte
	if b.whenMatchedUpdateAll && b.whenMatchedCondition != nil {
		options.WhenMatchedUpdateAllCondition, conditionData = cStringPtr(*b.whenMatchedCondition)
		defer runtime.KeepAlive(conditionData)
	}
	if b.whenNotMatchedDelete && b.whenNotMatchedFilter != nil {
		options.WhenNotMatchedBySourceDeleteFilter, filterData = cStringPtr(*b.whenNotMatchedFilter)
		defer runtime.KeepAlive(filterData)
	}
	if b.useIndex != nil {
		options.UseIndex = boolInt(*b.useIndex)
	}
	if b.timeoutMillis != nil {
		options.TimeoutMS = int64(*b.timeoutMillis)
	}
	onPtrs, keepOn := cStringArray(b.on)
	defer keepOn()

	var result native.MergeInsertResult
	var errorMessage *byte
	code := api.TableMergeInsertWithResult(t.handle, reader, onPtrs, uintptr(len(b.on)), &options, &result, &errorMessage)
	if err := check(api, code, errorMessage, "merge_insert failed"); err != nil {
		return nil, err
	}
	return &contracts.MergeResult{
		Version:         result.Version,
		NumInsertedRows: result.NumInsertedRows,
		NumUpdatedRows:  result.NumUpdatedRows,
		NumDeletedRows:  result.NumDeletedRows,
		NumAttempts:     result.NumAttempts,
	}, nil
}
