// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

import (
	"context"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
)

// IMergeInsertBuilder builds and executes a merge_insert (upsert) operation.
//
// Mirrors the Rust MergeInsertBuilder. Optional SQL conditions are passed as
// *string so callers can distinguish "no condition" (nil) from an empty string.
// Target rows are referenced in SQL conditions with the "target." prefix and
// source rows with "source." — e.g. "target.version < source.version".
type IMergeInsertBuilder interface {
	// WhenMatchedUpdateAll enables replacing matched target rows with the
	// corresponding source rows. If condition is non-nil, only matched rows
	// satisfying the SQL condition are updated.
	WhenMatchedUpdateAll(condition *string) IMergeInsertBuilder

	// WhenNotMatchedInsertAll enables inserting source rows that have no match
	// in the target table.
	WhenNotMatchedInsertAll() IMergeInsertBuilder

	// WhenNotMatchedBySourceDelete enables deleting target rows that have no
	// match in the source. If filter is non-nil, only target rows satisfying
	// the SQL filter are deleted.
	WhenNotMatchedBySourceDelete(filter *string) IMergeInsertBuilder

	// Timeout bounds the total time spent on the merge, including retries.
	Timeout(d time.Duration) IMergeInsertBuilder

	// UseIndex toggles use of the join-key index (defaults to true server-side).
	UseIndex(useIndex bool) IMergeInsertBuilder

	// Execute runs the merge with the given source records.
	Execute(ctx context.Context, records []arrow.Record) (*MergeResult, error)
}
