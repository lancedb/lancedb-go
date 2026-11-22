// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package native

// The types in this file mirror the public C structures in include/lancedb.h.
// They are used only behind pointers: purego calls must never pass them by
// value. layout_test.go pins their sizes and field offsets for 64-bit
// platforms, including the Windows LLP64 model.

// IpcBuffer mirrors LanceDBIpcBuffer.
type IpcBuffer struct {
	Data *byte
	Len  uintptr
}

// IndexOptions mirrors LanceDBIndexOptions. Numeric tuning fields use -1 for
// unset, as required by lancedb_index_options_default.
type IndexOptions struct {
	Name                *byte
	Replace             int32
	WaitTimeoutMS       uint64
	NumPartitions       int64
	SampleRate          int64
	MaxIterations       int64
	TargetPartitionSize int64
	NumSubVectors       int64
	NumBits             int64
	M                   int64
	EfConstruction      int64
	DistanceType        int32
	FTSBaseTokenizer    *byte
	FTSLanguage         *byte
	FTSMaxTokenLength   int64
	FTSLowerCase        int32
	FTSStem             int32
	FTSRemoveStopWords  int32
	FTSASCIIFolding     int32
	FTSWithPosition     int32
	FTSNgramMinLength   int64
	FTSNgramMaxLength   int64
	FTSNgramPrefixOnly  int32
}

// IndexInfo mirrors LanceDBIndexInfo.
type IndexInfo struct {
	Name       *byte
	Columns    **byte
	NumColumns uintptr
	IndexType  *byte
}

// IndexStats mirrors LanceDBIndexStats.
type IndexStats struct {
	NumIndexedRows   uintptr
	NumUnindexedRows uintptr
	NumIndices       uint32
}

// IndexStatistics mirrors LanceDBIndexStatistics.
type IndexStatistics struct {
	NumIndexedRows   uintptr
	NumUnindexedRows uintptr
	IndexType        *byte
	DistanceType     *byte
	NumIndices       uint32
	HasNumIndices    uint8
}

// Version mirrors LanceDBVersion.
type Version struct {
	Version          uint64
	TimestampSeconds int64
	TimestampNanos   uint32
}

// VersionMetadata mirrors LanceDBVersionMetadata.
type VersionMetadata struct {
	Keys   **byte
	Values **byte
	Count  uintptr
}

// Tag mirrors LanceDBTag.
type Tag struct {
	Name         *byte
	Version      uint64
	ManifestSize uint64
	Branch       *byte
}

// CompactionOptions mirrors LanceDBCompactionOptions.
type CompactionOptions struct {
	SetFields                     uint32
	TargetRowsPerFragment         uint64
	MaxRowsPerGroup               uint64
	MaxBytesPerFile               uint64
	MaterializeDeletions          int32
	MaterializeDeletionsThreshold float32
	NumThreads                    uint64
	BatchSize                     uint64
}

// PruneOptions mirrors LanceDBPruneOptions.
type PruneOptions struct {
	SetFields                uint32
	OlderThanNanos           int64
	DeleteUnverified         int32
	ErrorIfTaggedOldVersions int32
}

// OptimizeConfig mirrors LanceDBOptimizeConfig.
type OptimizeConfig struct {
	Action     int32
	Compaction CompactionOptions
	Prune      PruneOptions
}

// CompactionMetrics mirrors LanceDBCompactionMetrics.
type CompactionMetrics struct {
	FragmentsRemoved uintptr
	FragmentsAdded   uintptr
	FilesRemoved     uintptr
	FilesAdded       uintptr
}

// RemovalStats mirrors LanceDBRemovalStats.
type RemovalStats struct {
	BytesRemoved uint64
	OldVersions  uint64
}

// OptimizeStats mirrors LanceDBOptimizeStats.
type OptimizeStats struct {
	HasCompaction int32
	Compaction    CompactionMetrics
	HasPrune      int32
	Prune         RemovalStats
}

// MergeInsertOptions mirrors LanceDBMergeInsertOptions.
type MergeInsertOptions struct {
	WhenMatchedUpdateAll               int32
	WhenMatchedUpdateAllCondition      *byte
	WhenNotMatchedInsertAll            int32
	WhenNotMatchedBySourceDelete       int32
	WhenNotMatchedBySourceDeleteFilter *byte
	UseIndex                           int32
	TimeoutMS                          int64
}

// MergeInsertResult mirrors LanceDBMergeInsertResult.
type MergeInsertResult struct {
	Version         uint64
	NumInsertedRows uint64
	NumUpdatedRows  uint64
	NumDeletedRows  uint64
	NumAttempts     uint32
}

// MergeInsertResultV2 mirrors LanceDBMergeInsertResultV2.
type MergeInsertResultV2 struct {
	Version         uint64
	NumInsertedRows uint64
	NumUpdatedRows  uint64
	NumDeletedRows  uint64
	NumAttempts     uint32
	NumRows         uint64
}

// UpdateResult mirrors LanceDBUpdateResult.
type UpdateResult struct {
	RowsUpdated uint64
	Version     uint64
}

// DeleteResult mirrors LanceDBDeleteResult.
type DeleteResult struct {
	NumDeletedRows uint64
	Version        uint64
}

// ColumnAlteration mirrors LanceDBColumnAlteration.
type ColumnAlteration struct {
	Path        *byte
	Rename      *byte
	SetNullable int32
	Nullable    int32
}

// C error codes from LanceDBError.
const (
	ErrSuccess            int32 = 0
	ErrInvalidArgument    int32 = 1
	ErrInvalidTableName   int32 = 2
	ErrInvalidInput       int32 = 3
	ErrTableNotFound      int32 = 4
	ErrDatabaseNotFound   int32 = 5
	ErrIndexNotFound      int32 = 7
	ErrTableAlreadyExists int32 = 9
	ErrSchema             int32 = 11
	ErrTimeout            int32 = 13
	ErrNotSupported       int32 = 19
	ErrArrow              int32 = 18
)

// Distance metrics from LanceDBDistanceType.
const (
	DistanceL2      int32 = 0
	DistanceCosine  int32 = 1
	DistanceDot     int32 = 2
	DistanceHamming int32 = 3
)

// Index types from LanceDBIndexType.
const (
	IndexAuto      int32 = 0
	IndexBTree     int32 = 1
	IndexBitmap    int32 = 2
	IndexLabelList int32 = 3
	IndexFTS       int32 = 4
	IndexIVFFlat   int32 = 5
	IndexIVFPQ     int32 = 6
	IndexIVFHNSWPQ int32 = 7
	IndexIVFHNSWSQ int32 = 8
)

// Optimize actions from LanceDBOptimizeType.
const (
	OptimizeAll     int32 = 0
	OptimizeCompact int32 = 1
	OptimizePrune   int32 = 2
	OptimizeIndex   int32 = 3
)

// Normalization methods from LanceDBNormalizeMethod.
const (
	NormalizeScore int32 = 0
	NormalizeRank  int32 = 1
)

// Option bit fields.
const (
	CompactionTargetRowsPerFragment         uint32 = 1 << 0
	CompactionMaxRowsPerGroup               uint32 = 1 << 1
	CompactionMaxBytesPerFile               uint32 = 1 << 2
	CompactionMaterializeDeletions          uint32 = 1 << 3
	CompactionMaterializeDeletionsThreshold uint32 = 1 << 4
	CompactionNumThreads                    uint32 = 1 << 5
	CompactionBatchSize                     uint32 = 1 << 6

	PruneOlderThan                uint32 = 1 << 0
	PruneDeleteUnverified         uint32 = 1 << 1
	PruneErrorIfTaggedOldVersions uint32 = 1 << 2
)
