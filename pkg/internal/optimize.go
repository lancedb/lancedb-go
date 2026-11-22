// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func (t *Table) Optimize(ctx context.Context) (*contracts.OptimizeStats, error) {
	return t.OptimizeWithAction(ctx, contracts.OptimizeAction{Kind: contracts.OptimizeAll})
}

func (t *Table) OptimizeWithAction(_ context.Context, action contracts.OptimizeAction) (*contracts.OptimizeStats, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	config := native.OptimizeConfig{}
	switch action.Kind {
	case contracts.OptimizeAll:
		config.Action = native.OptimizeAll
	case contracts.OptimizeCompact:
		config.Action = native.OptimizeCompact
		applyCompaction(&config.Compaction, action.Compaction)
	case contracts.OptimizePrune:
		config.Action = native.OptimizePrune
		applyPrune(&config.Prune, action.Prune)
	case contracts.OptimizeIndex:
		config.Action = native.OptimizeIndex
	default:
		return nil, fmt.Errorf("unknown OptimizeActionKind: %d", action.Kind)
	}

	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var stats native.OptimizeStats
	var errorMessage *byte
	code := api.TableOptimizeWithAction(t.handle, &config, &stats, &errorMessage)
	if err := check(api, code, errorMessage, "failed to optimize table"); err != nil {
		return nil, err
	}
	out := &contracts.OptimizeStats{}
	if stats.HasCompaction != 0 {
		out.Compaction = &contracts.CompactionMetrics{
			FragmentsRemoved: int64Ptr(int64(stats.Compaction.FragmentsRemoved)),
			FragmentsAdded:   int64Ptr(int64(stats.Compaction.FragmentsAdded)),
			FilesRemoved:     int64Ptr(int64(stats.Compaction.FilesRemoved)),
			FilesAdded:       int64Ptr(int64(stats.Compaction.FilesAdded)),
		}
	}
	if stats.HasPrune != 0 {
		out.Prune = &contracts.RemovalStats{
			BytesRemoved: int64Ptr(int64(stats.Prune.BytesRemoved)),
			OldVersions:  int64Ptr(int64(stats.Prune.OldVersions)),
		}
	}
	return out, nil
}

func applyCompaction(options *native.CompactionOptions, params contracts.CompactionParams) {
	if params.TargetRowsPerFragment != nil {
		options.SetFields |= native.CompactionTargetRowsPerFragment
		options.TargetRowsPerFragment = *params.TargetRowsPerFragment
	}
	if params.MaxRowsPerGroup != nil {
		options.SetFields |= native.CompactionMaxRowsPerGroup
		options.MaxRowsPerGroup = *params.MaxRowsPerGroup
	}
	if params.MaxBytesPerFile != nil {
		options.SetFields |= native.CompactionMaxBytesPerFile
		options.MaxBytesPerFile = *params.MaxBytesPerFile
	}
	if params.MaterializeDeletions != nil {
		options.SetFields |= native.CompactionMaterializeDeletions
		options.MaterializeDeletions = boolInt(*params.MaterializeDeletions)
	}
	if params.MaterializeDeletionsThreshold != nil {
		options.SetFields |= native.CompactionMaterializeDeletionsThreshold
		options.MaterializeDeletionsThreshold = *params.MaterializeDeletionsThreshold
	}
	if params.NumThreads != nil {
		options.SetFields |= native.CompactionNumThreads
		options.NumThreads = *params.NumThreads
	}
	if params.BatchSize != nil {
		options.SetFields |= native.CompactionBatchSize
		options.BatchSize = *params.BatchSize
	}
}

func applyPrune(options *native.PruneOptions, params contracts.PruneParams) {
	if params.OlderThan > 0 {
		seconds := uint64(params.OlderThan / time.Second)
		if seconds == 0 {
			seconds = 1
		}
		options.SetFields |= native.PruneOlderThan
		options.OlderThanNanos = int64(seconds) * int64(time.Second)
	}
	if params.DeleteUnverified != nil {
		options.SetFields |= native.PruneDeleteUnverified
		options.DeleteUnverified = boolInt(*params.DeleteUnverified)
	}
	if params.ErrorIfTaggedOldVersions != nil {
		options.SetFields |= native.PruneErrorIfTaggedOldVersions
		options.ErrorIfTaggedOldVersions = boolInt(*params.ErrorIfTaggedOldVersions)
	}
}

func int64Ptr(value int64) *int64 { return &value }
