// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
)

func TestRustDebugTypeNestedLegacyList(t *testing.T) {
	dataType := arrow.ListOf(arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32))
	want := `List(Field { name: "item", data_type: FixedSizeList(Field { name: "item", data_type: Float32, nullable: true, dict_id: 0, dict_is_ordered: false, metadata: {} }, 2), nullable: true, dict_id: 0, dict_is_ordered: false, metadata: {} })`
	if got := rustDebugType(dataType); got != want {
		t.Fatalf("rustDebugType() = %q, want %q", got, want)
	}
}
