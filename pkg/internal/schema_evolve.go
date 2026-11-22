// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func (t *Table) AddColumns(_ context.Context, transforms []contracts.NewColumnTransform) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	if len(transforms) == 0 {
		return 0, fmt.Errorf("add_columns: transforms must be non-empty")
	}
	names := make([]string, len(transforms))
	expressions := make([]string, len(transforms))
	for i, transform := range transforms {
		if strings.TrimSpace(transform.Name) == "" {
			return 0, fmt.Errorf("add_columns: transforms[%d].Name is empty", i)
		}
		if strings.TrimSpace(transform.Expression) == "" {
			return 0, fmt.Errorf("add_columns: transforms[%d].Expression is empty", i)
		}
		names[i] = transform.Name
		expressions[i] = transform.Expression
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	namePtrs, keepNames := cStringArray(names)
	defer keepNames()
	expressionPtrs, keepExpressions := cStringArray(expressions)
	defer keepExpressions()
	var version uint64
	var errorMessage *byte
	code := api.TableAddColumns(t.handle, namePtrs, expressionPtrs, uintptr(len(names)), &version, &errorMessage)
	if err := check(api, code, errorMessage, "failed to add columns"); err != nil {
		return 0, err
	}
	return version, nil
}

func (t *Table) AlterColumns(_ context.Context, alterations []contracts.ColumnAlteration) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	if len(alterations) == 0 {
		return 0, fmt.Errorf("alter_columns: alterations must be non-empty")
	}
	for i, alteration := range alterations {
		if strings.TrimSpace(alteration.Path) == "" {
			return 0, fmt.Errorf("alter_columns: alterations[%d].Path is empty", i)
		}
		if alteration.Rename == nil && alteration.Nullable == nil {
			return 0, fmt.Errorf("alter_columns: alterations[%d] has no rename or nullable change", i)
		}
		if alteration.Rename != nil && strings.TrimSpace(*alteration.Rename) == "" {
			return 0, fmt.Errorf("alter_columns: alterations[%d].Rename is empty string", i)
		}
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	items := make([]native.ColumnAlteration, len(alterations))
	keep := make([][]byte, 0, len(alterations)*2)
	for i, alteration := range alterations {
		pathData := cString(alteration.Path)
		keep = append(keep, pathData)
		items[i].Path = &pathData[0]
		if alteration.Rename != nil {
			renameData := cString(*alteration.Rename)
			keep = append(keep, renameData)
			items[i].Rename = &renameData[0]
		}
		if alteration.Nullable != nil {
			items[i].SetNullable = 1
			items[i].Nullable = boolInt(*alteration.Nullable)
		}
	}
	defer runtime.KeepAlive(keep)
	var version uint64
	var errorMessage *byte
	code := api.TableAlterColumns(t.handle, &items[0], uintptr(len(items)), &version, &errorMessage)
	if err := check(api, code, errorMessage, "failed to alter columns"); err != nil {
		return 0, err
	}
	return version, nil
}

func (t *Table) DropColumns(_ context.Context, names []string) (uint64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("drop_columns: names must be non-empty")
	}
	for i, name := range names {
		if strings.TrimSpace(name) == "" {
			return 0, fmt.Errorf("drop_columns: names[%d] is empty", i)
		}
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	ptr, keep := cStringArray(names)
	defer keep()
	var version uint64
	var errorMessage *byte
	code := api.TableDropColumns(t.handle, ptr, uintptr(len(names)), &version, &errorMessage)
	if err := check(api, code, errorMessage, "failed to drop columns"); err != nil {
		return 0, err
	}
	return version, nil
}
