// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/stretchr/testify/require"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// u32Ptr is a tiny helper for the optional IndexParams fields.
func u32Ptr(v uint32) *uint32 { return &v }
func boolPtr(v bool) *bool    { return &v }

// setupIndexBuilderTable seeds 300 rows with a 64-dim embedding and a
// stringy text column so the test can exercise both vector and FTS index
// variants against the same table.
func setupIndexBuilderTable(t *testing.T) (*internal.Table, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "lancedb_test_index_builder_")
	require.NoError(t, err)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("connect: %v", err)
	}

	fields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "embedding", Type: arrow.FixedSizeListOf(64, arrow.PrimitiveTypes.Float32), Nullable: false},
	}
	arrowSchema := arrow.NewSchema(fields, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("schema: %v", err)
	}
	table, err := conn.CreateTable(context.Background(), "idx_v2", schema)
	if err != nil {
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("create table: %v", err)
	}

	const n = 300
	pool := memory.NewGoAllocator()
	idB := array.NewInt32Builder(pool)
	txtB := array.NewStringBuilder(pool)
	embB := array.NewFixedSizeListBuilder(pool, 64, arrow.PrimitiveTypes.Float32)
	embValB := embB.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < n; i++ {
		idB.Append(int32(i))
		txtB.Append("the quick brown fox " + string(rune('a'+i%26)))
		embB.Append(true)
		for j := 0; j < 64; j++ {
			embValB.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	rec := array.NewRecord(arrowSchema, []arrow.Array{idB.NewArray(), txtB.NewArray(), embB.NewArray()}, n)
	defer rec.Release()
	if err := table.Add(context.Background(), rec, nil); err != nil {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("add: %v", err)
	}

	cleanup := func() {
		table.Close()
		conn.Close()
		os.RemoveAll(tempDir)
	}
	return table.(*internal.Table), cleanup
}

// TestCreateIndexWithParams_IvfPq_FullTuning — IVF_PQ with explicit
// num_partitions / num_sub_vectors / distance_type + custom name. Verifies
// the index shows up under the custom name.
func TestCreateIndexWithParams_IvfPq_FullTuning(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		contracts.IndexParams{
			NumPartitions: u32Ptr(8),
			NumSubVectors: u32Ptr(4),
			NumBits:       u32Ptr(8),
			DistanceType:  contracts.DistanceTypeCosine,
		},
		&contracts.CreateIndexOptions{
			Name:        "emb_ivfpq",
			Replace:     false,
			WaitTimeout: 30 * time.Second,
		},
	)
	require.NoError(t, err)

	indexes, err := table.GetAllIndexes(context.Background())
	require.NoError(t, err)

	found := false
	for _, ix := range indexes {
		if ix.Name == "emb_ivfpq" {
			found = true
			break
		}
	}
	require.True(t, found, "expected an index named emb_ivfpq; got %v", indexes)
}

// TestCreateIndexWithParams_HnswPq_TuningKnobs — IVF_HNSW_PQ with m /
// ef_construction / num_sub_vectors set. Smoke-tests the HNSW tuning
// parameters reach the FFI.
func TestCreateIndexWithParams_HnswPq_TuningKnobs(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeHnswPq,
		contracts.IndexParams{
			NumPartitions:  u32Ptr(4),
			M:              u32Ptr(8),
			EfConstruction: u32Ptr(64),
			NumSubVectors:  u32Ptr(4),
			DistanceType:   contracts.DistanceTypeL2,
		},
		&contracts.CreateIndexOptions{WaitTimeout: 30 * time.Second},
	)
	require.NoError(t, err)
}

// TestCreateIndexWithParams_FTS_FullTuning — FTS with with_position,
// language, stem, remove_stop_words tuning. Index build must succeed and
// the index must be discoverable via GetAllIndexes.
func TestCreateIndexWithParams_FTS_FullTuning(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"text"},
		contracts.IndexTypeFts,
		contracts.IndexParams{
			FtsLanguage:        "English",
			FtsWithPosition:    boolPtr(true),
			FtsStem:            boolPtr(true),
			FtsRemoveStopWords: boolPtr(true),
			FtsLowerCase:       boolPtr(true),
		},
		&contracts.CreateIndexOptions{Name: "text_fts", WaitTimeout: 30 * time.Second},
	)
	require.NoError(t, err)

	indexes, err := table.GetAllIndexes(context.Background())
	require.NoError(t, err)
	found := false
	for _, ix := range indexes {
		if ix.Name == "text_fts" {
			found = true
			break
		}
	}
	require.True(t, found, "expected an index named text_fts; got %v", indexes)
}

// TestCreateIndexWithParams_Replace_OverridesExisting — creating an index
// with the same name twice fails unless Replace is true.
func TestCreateIndexWithParams_Replace_OverridesExisting(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	params := contracts.IndexParams{NumPartitions: u32Ptr(8), NumSubVectors: u32Ptr(4)}

	require.NoError(t, table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		params,
		&contracts.CreateIndexOptions{Name: "emb_idx", WaitTimeout: 30 * time.Second},
	))

	// Second call without Replace should error (index already exists).
	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		params,
		&contracts.CreateIndexOptions{Name: "emb_idx", WaitTimeout: 30 * time.Second},
	)
	require.Error(t, err, "duplicate index without Replace must fail")

	// Same call with Replace=true succeeds.
	require.NoError(t, table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		params,
		&contracts.CreateIndexOptions{Name: "emb_idx", Replace: true, WaitTimeout: 30 * time.Second},
	))
}

// TestCreateIndexWithParams_UnknownType_Error — defense against an
// unmapped IndexType slipping through. The indexTypeToString default
// returns "vector" which is valid, so this exercises the JSON type path
// via a direct-on-Rust guard: pass Ivf params on a BTree column combo
// that cannot be trained.
func TestCreateIndexWithParams_NilOptsIsEquivalentToDefault(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	// nil opts must not panic; treated as zero CreateIndexOptions.
	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		contracts.IndexParams{NumPartitions: u32Ptr(4), NumSubVectors: u32Ptr(4)},
		nil,
	)
	require.NoError(t, err)
}

// TestCreateIndexWithParams_InvalidDistanceType_ReturnsError — Strategy 1
// (Edge): an out-of-range cast like contracts.DistanceType(99) used to
// panic via distanceTypeToString. Pin the error path so callers passing
// decoded/cast values get a normal error instead of a process crash.
func TestCreateIndexWithParams_InvalidDistanceType_ReturnsError(t *testing.T) {
	table, cleanup := setupIndexBuilderTable(t)
	defer cleanup()

	err := table.CreateIndexWithParams(
		context.Background(),
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		contracts.IndexParams{
			NumPartitions: u32Ptr(4),
			NumSubVectors: u32Ptr(4),
			DistanceType:  contracts.DistanceType(99),
		},
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DistanceType")
}
