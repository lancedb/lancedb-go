// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"bytes"
	"fmt"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

// ipcBytesToRecord deserializes Arrow IPC bytes into a single arrow.Record.
// If the IPC file contains multiple batches, they are concatenated.
// Returns nil, nil for nil/empty input (empty result set).
// The caller is responsible for calling Release() on the returned Record (if non-nil).
func ipcBytesToRecord(ipcBytes []byte) (arrow.Record, error) {
	if len(ipcBytes) == 0 {
		return nil, nil
	}

	reader, err := ipc.NewFileReader(bytes.NewReader(ipcBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create IPC reader: %w", err)
	}
	defer reader.Close()

	schema := reader.Schema()
	numRecords := reader.NumRecords()

	if numRecords == 0 {
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

	if numRecords == 1 {
		rec, err := reader.Record(0)
		if err != nil {
			return nil, fmt.Errorf("failed to read record batch: %w", err)
		}
		rec.Retain()
		return rec, nil
	}

	// Multiple batches — collect and concatenate
	records := make([]arrow.Record, 0, numRecords)
	for i := 0; i < numRecords; i++ {
		rec, err := reader.Record(i)
		if err != nil {
			return nil, fmt.Errorf("failed to read record batch %d: %w", i, err)
		}
		rec.Retain()
		records = append(records, rec)
	}
	defer func() {
		for _, r := range records {
			r.Release()
		}
	}()

	table := array.NewTableFromRecords(schema, records)
	defer table.Release()

	tr := array.NewTableReader(table, table.NumRows())
	defer tr.Release()
	if tr.Next() {
		rec := tr.Record()
		rec.Retain()
		return rec, nil
	}

	return nil, fmt.Errorf("failed to concatenate record batches")
}
