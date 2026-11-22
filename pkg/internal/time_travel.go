// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func (t *Table) ListVersions(_ context.Context) ([]contracts.VersionInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var versions *native.Version
	var metadata *native.VersionMetadata
	var count uintptr
	var errorMessage *byte
	code := api.TableListVersions(t.handle, &versions, &metadata, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to list versions"); err != nil {
		if versions != nil {
			api.FreeVersions(versions, count)
		}
		if metadata != nil {
			api.FreeVersionMetadata(metadata, count)
		}
		return nil, err
	}
	if versions == nil || count == 0 {
		if versions != nil {
			api.FreeVersions(versions, count)
		}
		if metadata != nil {
			api.FreeVersionMetadata(metadata, count)
		}
		return []contracts.VersionInfo{}, nil
	}
	defer api.FreeVersions(versions, count)
	if metadata != nil {
		defer api.FreeVersionMetadata(metadata, count)
	}

	versionItems := unsafe.Slice(versions, int(count))
	var metadataItems []native.VersionMetadata
	if metadata != nil {
		metadataItems = unsafe.Slice(metadata, int(count))
	}
	out := make([]contracts.VersionInfo, len(versionItems))
	for i, version := range versionItems {
		info := contracts.VersionInfo{
			Version:   version.Version,
			Timestamp: time.Unix(version.TimestampSeconds, int64(version.TimestampNanos)).UTC(),
		}
		if i < len(metadataItems) {
			info.Metadata = metadataMap(metadataItems[i])
		}
		out[i] = info
	}
	return out, nil
}

func metadataMap(metadata native.VersionMetadata) map[string]string {
	if metadata.Count == 0 || metadata.Keys == nil || metadata.Values == nil {
		return nil
	}
	keys := unsafe.Slice(metadata.Keys, int(metadata.Count))
	values := unsafe.Slice(metadata.Values, int(metadata.Count))
	out := make(map[string]string, len(keys))
	for i := range keys {
		out[goString(keys[i])] = goString(values[i])
	}
	return out
}

func (t *Table) Checkout(_ context.Context, version uint64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableCheckout(t.handle, version, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to checkout version %d", version))
}

func (t *Table) CheckoutTag(_ context.Context, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableCheckoutTag(t.handle, tag, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to checkout tag %q", tag))
}

func (t *Table) CheckoutLatest(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableCheckoutLatest(t.handle, &errorMessage)
	return check(api, code, errorMessage, "failed to checkout latest")
}

func (t *Table) Restore(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableRestore(t.handle, &errorMessage)
	return check(api, code, errorMessage, "failed to restore")
}

func (t *Table) TagList(_ context.Context) (map[string]contracts.TagInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var tags *native.Tag
	var count uintptr
	var errorMessage *byte
	code := api.TableListTags(t.handle, &tags, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to list tags"); err != nil {
		if tags != nil {
			api.FreeTags(tags, count)
		}
		return nil, err
	}
	if tags == nil || count == 0 {
		if tags != nil {
			api.FreeTags(tags, count)
		}
		return map[string]contracts.TagInfo{}, nil
	}
	defer api.FreeTags(tags, count)
	items := unsafe.Slice(tags, int(count))
	out := make(map[string]contracts.TagInfo, len(items))
	for _, tag := range items {
		name := goString(tag.Name)
		out[name] = contracts.TagInfo{
			Version:      tag.Version,
			ManifestSize: tag.ManifestSize,
			Branch:       goString(tag.Branch),
		}
	}
	return out, nil
}

func (t *Table) TagGetVersion(_ context.Context, tag string) (uint64, error) {
	if tag == "" {
		return 0, fmt.Errorf("tag name cannot be empty")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return 0, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return 0, err
	}
	var version uint64
	var errorMessage *byte
	code := api.TableGetTagVersion(t.handle, tag, &version, &errorMessage)
	if err := check(api, code, errorMessage, fmt.Sprintf("failed to resolve tag %q", tag)); err != nil {
		return 0, err
	}
	return version, nil
}

func (t *Table) TagCreate(_ context.Context, tag string, version uint64) error {
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableCreateTag(t.handle, tag, version, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to create tag %q", tag))
}

func (t *Table) TagDelete(_ context.Context, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableDeleteTag(t.handle, tag, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to delete tag %q", tag))
}

func (t *Table) TagUpdate(_ context.Context, tag string, version uint64) error {
	if tag == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	var errorMessage *byte
	code := api.TableUpdateTag(t.handle, tag, version, &errorMessage)
	return check(api, code, errorMessage, fmt.Sprintf("failed to update tag %q", tag))
}
