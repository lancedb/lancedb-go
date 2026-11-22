// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal/native"
)

func (t *Table) CreateIndex(ctx context.Context, columns []string, indexType contracts.IndexType) error {
	return t.CreateIndexWithName(ctx, columns, indexType, "")
}

func (t *Table) CreateIndexWithName(_ context.Context, columns []string, indexType contracts.IndexType, name string) error {
	if len(columns) == 0 {
		return fmt.Errorf("columns list cannot be empty")
	}
	options := defaultIndexOptions()
	if name != "" {
		options.Name = nil // set below after allocating the C string
	}
	return t.createIndex(columns, indexType, &options, name)
}

func (t *Table) CreateIndexWithParams(_ context.Context, columns []string, indexType contracts.IndexType, params contracts.IndexParams, opts *contracts.CreateIndexOptions) error {
	if len(columns) == 0 {
		return fmt.Errorf("columns list cannot be empty")
	}
	options := defaultIndexOptions()
	options.Replace = 0
	if opts != nil {
		options.Replace = boolInt(opts.Replace)
		if opts.WaitTimeout > 0 {
			options.WaitTimeoutMS = uint64(opts.WaitTimeout.Milliseconds())
		}
	}
	if params.NumPartitions != nil {
		options.NumPartitions = int64(*params.NumPartitions)
	}
	if params.SampleRate != nil {
		options.SampleRate = int64(*params.SampleRate)
	}
	if params.MaxIterations != nil {
		options.MaxIterations = int64(*params.MaxIterations)
	}
	if params.TargetPartitionSize != nil {
		options.TargetPartitionSize = int64(*params.TargetPartitionSize)
	}
	if params.NumSubVectors != nil {
		options.NumSubVectors = int64(*params.NumSubVectors)
	}
	if params.NumBits != nil {
		options.NumBits = int64(*params.NumBits)
	}
	if params.M != nil {
		options.M = int64(*params.M)
	}
	if params.EfConstruction != nil {
		options.EfConstruction = int64(*params.EfConstruction)
	}
	if params.DistanceType != nil {
		distance, err := distanceTypeToNative(*params.DistanceType)
		if err != nil {
			return err
		}
		options.DistanceType = distance
	}
	if params.FtsBaseTokenizer != "" {
		options.FTSBaseTokenizer = nil // set below
	}
	if params.FtsLanguage != "" {
		options.FTSLanguage = nil // set below
	}
	if params.FtsMaxTokenLength != nil {
		options.FTSMaxTokenLength = int64(*params.FtsMaxTokenLength)
	}
	if params.FtsLowerCase != nil {
		options.FTSLowerCase = boolInt(*params.FtsLowerCase)
	}
	if params.FtsStem != nil {
		options.FTSStem = boolInt(*params.FtsStem)
	}
	if params.FtsRemoveStopWords != nil {
		options.FTSRemoveStopWords = boolInt(*params.FtsRemoveStopWords)
	}
	if params.FtsASCIIFolding != nil {
		options.FTSASCIIFolding = boolInt(*params.FtsASCIIFolding)
	}
	if params.FtsWithPosition != nil {
		options.FTSWithPosition = boolInt(*params.FtsWithPosition)
	}
	if params.FtsNgramMinLength != nil {
		options.FTSNgramMinLength = int64(*params.FtsNgramMinLength)
	}
	if params.FtsNgramMaxLength != nil {
		options.FTSNgramMaxLength = int64(*params.FtsNgramMaxLength)
	}
	if params.FtsNgramPrefixOnly != nil {
		options.FTSNgramPrefixOnly = boolInt(*params.FtsNgramPrefixOnly)
	}
	name := ""
	if opts != nil {
		name = opts.Name
	}
	return t.createIndexWithStrings(columns, indexType, &options, name, params.FtsBaseTokenizer, params.FtsLanguage)
}

func (t *Table) createIndex(columns []string, indexType contracts.IndexType, options *native.IndexOptions, name string) error {
	return t.createIndexWithStrings(columns, indexType, options, name, "", "")
}

func (t *Table) createIndexWithStrings(columns []string, indexType contracts.IndexType, options *native.IndexOptions, name, baseTokenizer, language string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return err
	}
	columnPtrs, keepColumns := cStringArray(columns)
	defer keepColumns()

	var nameData, tokenizerData, languageData []byte
	if name != "" {
		options.Name, nameData = cStringPtr(name)
		defer runtime.KeepAlive(nameData)
	}
	if baseTokenizer != "" {
		options.FTSBaseTokenizer, tokenizerData = cStringPtr(baseTokenizer)
		defer runtime.KeepAlive(tokenizerData)
	}
	if language != "" {
		options.FTSLanguage, languageData = cStringPtr(language)
		defer runtime.KeepAlive(languageData)
	}
	var errorMessage *byte
	code := api.TableCreateIndex(t.handle, columnPtrs, uintptr(len(columns)), indexTypeToNative(indexType), options, &errorMessage)
	return check(api, code, errorMessage, "failed to create index")
}

func (t *Table) GetAllIndexes(_ context.Context) ([]contracts.IndexInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var indices *native.IndexInfo
	var count uintptr
	var errorMessage *byte
	code := api.TableListIndicesInfo(t.handle, &indices, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to list indexes"); err != nil {
		if indices != nil {
			api.FreeIndexInfo(indices, count)
		}
		return nil, err
	}
	if indices == nil || count == 0 {
		if indices != nil {
			api.FreeIndexInfo(indices, count)
		}
		return []contracts.IndexInfo{}, nil
	}
	defer api.FreeIndexInfo(indices, count)
	items := unsafeSliceIndexInfo(indices, count)
	result := make([]contracts.IndexInfo, len(items))
	for i, item := range items {
		result[i] = contracts.IndexInfo{
			Name:      goString(item.Name),
			Columns:   goStringArray(item.Columns, item.NumColumns),
			IndexType: indexTypeStringToGo(goString(item.IndexType)),
		}
	}
	return result, nil
}

func (t *Table) IndexStats(_ context.Context, indexName string) (*contracts.IndexStatistics, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed || t.handle == 0 {
		return nil, fmt.Errorf("table is closed")
	}
	api, err := loadAPI()
	if err != nil {
		return nil, err
	}
	var stats *native.IndexStatistics
	var errorMessage *byte
	code := api.TableIndexStatistics(t.handle, indexName, &stats, &errorMessage)
	if err := check(api, code, errorMessage, "failed to get index statistics"); err != nil {
		return nil, err
	}
	if stats == nil {
		return nil, nil
	}
	defer api.FreeIndexStatistics(stats)
	out := &contracts.IndexStatistics{
		NumIndexedRows:   int64(stats.NumIndexedRows),
		NumUnindexedRows: int64(stats.NumUnindexedRows),
		IndexType:        indexTypeStringToGo(goString(stats.IndexType)),
	}
	if stats.DistanceType != nil {
		distance := goString(stats.DistanceType)
		out.DistanceType = &distance
	}
	if stats.HasNumIndices != 0 {
		value := int(stats.NumIndices)
		out.NumIndices = &value
	}
	return out, nil
}

func (t *Table) DropIndex(_ context.Context, name string) error {
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
	code := api.TableDropIndex(t.handle, name, &errorMessage)
	return check(api, code, errorMessage, "failed to drop index")
}

func (t *Table) PrewarmIndex(_ context.Context, name string) error {
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
	code := api.TablePrewarmIndex(t.handle, name, &errorMessage)
	return check(api, code, errorMessage, "failed to prewarm index")
}

func (t *Table) WaitForIndex(ctx context.Context, names []string, timeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
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
	if len(names) == 0 {
		// The documented v0.1.2 behavior waits for every index when names is
		// empty. The SDK requires explicit names, so take one snapshot and
		// wait for that set.
		listed, err := listIndexNames(api, t.handle)
		if err != nil {
			return fmt.Errorf("wait_for_index failed: %w", err)
		}
		if len(listed) == 0 {
			return nil
		}
		names = listed
	}
	namePtrs, keep := cStringArray(names)
	defer keep()
	var errorMessage *byte
	code := api.TableWaitForIndex(t.handle, namePtrs, uintptr(len(names)), ComputeWaitTimeoutMs(ctx, timeout), &errorMessage)
	return check(api, code, errorMessage, "wait_for_index failed")
}

func listIndexNames(api *native.API, table uintptr) ([]string, error) {
	var indices *native.IndexInfo
	var count uintptr
	var errorMessage *byte
	code := api.TableListIndicesInfo(table, &indices, &count, &errorMessage)
	if err := check(api, code, errorMessage, "failed to list indexes"); err != nil {
		return nil, err
	}
	if indices == nil || count == 0 {
		if indices != nil {
			api.FreeIndexInfo(indices, count)
		}
		return nil, nil
	}
	defer api.FreeIndexInfo(indices, count)
	items := unsafe.Slice(indices, int(count))
	names := make([]string, 0, len(items))
	for _, item := range items {
		if name := goString(item.Name); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// ComputeWaitTimeoutMs folds the caller timeout and any context deadline into
// one millisecond budget. Zero means no upper bound. Positive sub-millisecond
// budgets floor to one millisecond instead of becoming unbounded.
func ComputeWaitTimeoutMs(ctx context.Context, timeout time.Duration) uint64 {
	if timeout < 0 {
		return 1
	}
	effective := timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 1
		}
		if effective == 0 || remaining < effective {
			effective = remaining
		}
	}
	if effective == 0 {
		return 0
	}
	milliseconds := effective.Milliseconds()
	if milliseconds < 1 {
		milliseconds = 1
	}
	return uint64(milliseconds)
}

func defaultIndexOptions() native.IndexOptions {
	return native.IndexOptions{
		Replace:             -1,
		NumPartitions:       -1,
		SampleRate:          -1,
		MaxIterations:       -1,
		TargetPartitionSize: -1,
		NumSubVectors:       -1,
		NumBits:             -1,
		M:                   -1,
		EfConstruction:      -1,
		DistanceType:        -1,
		FTSMaxTokenLength:   -1,
		FTSLowerCase:        -1,
		FTSStem:             -1,
		FTSRemoveStopWords:  -1,
		FTSASCIIFolding:     -1,
		FTSWithPosition:     -1,
		FTSNgramMinLength:   -1,
		FTSNgramMaxLength:   -1,
		FTSNgramPrefixOnly:  -1,
	}
}

func boolInt(value bool) int32 {
	if value {
		return 1
	}
	return 0
}

func indexTypeToNative(indexType contracts.IndexType) int32 {
	switch indexType {
	case contracts.IndexTypeIvfPq:
		return native.IndexIVFPQ
	case contracts.IndexTypeIvfFlat:
		return native.IndexIVFFlat
	case contracts.IndexTypeHnswPq:
		return native.IndexIVFHNSWPQ
	case contracts.IndexTypeHnswSq:
		return native.IndexIVFHNSWSQ
	case contracts.IndexTypeBTree:
		return native.IndexBTree
	case contracts.IndexTypeBitmap:
		return native.IndexBitmap
	case contracts.IndexTypeLabelList:
		return native.IndexLabelList
	case contracts.IndexTypeFts:
		return native.IndexFTS
	case contracts.IndexTypeAuto:
		// v0.1.2 mapped Auto to the vector IVF-PQ path.
		return native.IndexIVFPQ
	default:
		return native.IndexIVFPQ
	}
}

func indexTypeStringToGo(indexType string) string {
	switch strings.ToUpper(indexType) {
	case "IVF_PQ", "IVFPQ":
		return "IvfPq"
	case "IVF_FLAT", "IVFFLAT":
		return "IvfFlat"
	case "IVF_HNSW_PQ", "IVFHNSWPQ":
		return "IvfHnswPq"
	case "IVF_HNSW_SQ", "IVFHNSWSQ":
		return "IvfHnswSq"
	case "BTREE":
		return "BTree"
	case "BITMAP":
		return "Bitmap"
	case "LABEL_LIST", "LABELLIST":
		return "LabelList"
	case "FTS":
		return "Fts"
	case "AUTO":
		return "Auto"
	default:
		return indexType
	}
}

func unsafeSliceIndexInfo(ptr *native.IndexInfo, count uintptr) []native.IndexInfo {
	if ptr == nil || count == 0 {
		return nil
	}
	return unsafe.Slice(ptr, int(count))
}
