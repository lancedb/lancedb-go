// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"testing"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
)

func distanceTypePtr(value contracts.DistanceType) *contracts.DistanceType { return &value }

func asMergeInsertTable(t *testing.T, table contracts.ITable) *internal.Table {
	t.Helper()
	concrete, ok := table.(*internal.Table)
	if !ok {
		t.Fatalf("table is %T, want *internal.Table", table)
	}
	return concrete
}

func asIndexManagement(t *testing.T, table contracts.ITable) contracts.ITableIndexManagement {
	t.Helper()
	concrete, ok := table.(contracts.ITableIndexManagement)
	if !ok {
		t.Fatalf("table is %T, want contracts.ITableIndexManagement", table)
	}
	return concrete
}

func asOptimize(t *testing.T, table contracts.ITable) contracts.ITableOptimize {
	t.Helper()
	concrete, ok := table.(contracts.ITableOptimize)
	if !ok {
		t.Fatalf("table is %T, want contracts.ITableOptimize", table)
	}
	return concrete
}
