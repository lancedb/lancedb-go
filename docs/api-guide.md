# Go API guide

This guide covers the pure-Go preview line of the LanceDB Go SDK. It documents
the exported surface in `pkg/lancedb` and `pkg/contracts`, then shows complete
programs for the operations the SDK supports. Every program in this file is
self-contained, compiles with `CGO_ENABLED=0` on Go 1.21, and was run against
the published native payload. The maintained programs live in
[`examples/`](../examples/); the tests that pin the contracts are linked
from each section.

The authoritative symbol reference is the [module
documentation](https://pkg.go.dev/github.com/lancedb/lancedb-go) for the version
you build against. The release's exact native chain belongs in its release
notes; this guide stays version-independent.

## Topics

- Setup: [Preview constraints](#preview-constraints),
  [Running the examples](#running-the-examples),
  [Compatibility model](#compatibility-model)
- Data: [Connections and schemas](#connections-and-schemas),
  [Queries](#queries), [Vector search](#vector-search),
  [Full-text and hybrid search](#full-text-and-hybrid-search),
  [Mutations](#mutations), [Merge insert](#merge-insert),
  [Schema evolution](#schema-evolution), [Time travel](#time-travel)
- Maintenance: [Index management and optimization](#index-management-and-optimization),
  [Cloud storage](#cloud-storage)
- Contracts: [SQL dialect](#sql-dialect),
  [Arrow ownership and lifetime](#arrow-ownership-and-lifetime),
  [Errors and context](#errors-and-context),
  [Exported surface](#exported-surface)

## Preview constraints

- **Pure Go runtime.** The SDK calls the published `lancedb-c` shared library
  through `purego`. There is no Rust toolchain, no C compiler and no cgo in the
  consumer build. Build with `CGO_ENABLED=0`.
- **Go 1.21 and Arrow Go v17.** The module declares `go 1.21.0`; the CI matrix
  uses Go 1.21.13. All Arrow values use `github.com/apache/arrow/go/v17/arrow`
  types. Arrow Go v16 or v18 values are not type-compatible.
- **Supported targets.** `linux/amd64`, `linux/arm64`, `darwin/arm64`,
  `windows/amd64` and `windows/arm64`. Intel macOS (`darwin/amd64`) is not
  supported.
- **Linux runtime.** The published Linux libraries require glibc 2.35 or
  newer. A 2.34 loader fails with `version 'GLIBC_2.35' not found`.
- **Preview versioning.** `@latest` still resolves the stable `v0.1.2` line,
  which predates the pure-Go preview and has no native payload or
  `lancedb-go-native` helper. Select the preview from the
  [releases page](https://github.com/lancedb/lancedb-go/releases) explicitly.
- **Module-backed native loader.** The native library for the build target is
  a compiled-in companion module
  (`go.lancedb.io/lancedb-go/artifacts/<target>`). The first `Connect`
  verifies its payload against the published SHA-256 digests and extracts it
  into a private per-user cache. No network access happens at runtime, and the
  library is never unloaded. `LANCEDB_C_LIBRARY_PATH` selects an explicit
  library file and is the only candidate when set; `LANCEDB_C_CACHE_DIR`
  overrides the cache root. `lancedb.RuntimeVersion()` reports the loaded
  library's version.
- **Published API preserved.** The v0.1.2 interfaces and struct layouts keep
  their signatures and field layouts. New capabilities live on companion
  interfaces, detected with a type assertion, because Go cannot overload
  `Execute` by context argument or return type.
- **SQL dialect.** Predicates use the LanceDB SDK dialect: double quotes
  delimit column identifiers and single quotes delimit string literals. See
  [SQL dialect](#sql-dialect) and [MIGRATION.md](../MIGRATION.md).

## Running the examples

Each program below is a complete `package main`. To run one, put it in its own
directory and add the SDK at the preview version you use:

```bash
mkdir lancedb-example && cd lancedb-example
go mod init example.com/lancedb-example
PREVIEW=vX.Y.Z-preview.N   # replace with the preview tag from the releases page
go get github.com/lancedb/lancedb-go@$PREVIEW
go run .
```

The programs create temporary local databases, so they need no service. The
cloud storage program uses environment variables for its optional loopback S3
fixture and prints placeholders otherwise. Never put real credentials in a
program or in a repository.

## Compatibility model

`ITable` is the published v0.1.2 interface. The native table implements it and
the following companion interfaces:

| Interface | Capability |
| --- | --- |
| `ITableQueryV2` | Context/single-record query builder, `SelectV2`, `SelectIPCV2`. |
| `ITableVectorQuery` | Vector query builder with tuning and hybrid search. |
| `ITableIndexManagement` | Parameterized index creation, stats, wait, drop. |
| `ITableMergeInsert` | Merge-insert (upsert) builder. |
| `ITableOptimize` | Compact, prune and index optimization with stats. |
| `ITableUpdateExpr` | Raw SQL update expressions with affected-row and version results. |
| `ITableTimeTravel` | Versions, checkout, restore and tags. |
| `ITableSchemaEvolve` | Add, alter and drop columns. |
| `ITablePrewarmIndex` | Prewarm a named index. |

Detect a capability with a normal type assertion. Do not expand the published
interfaces:

```go
table, err := db.OpenTable(ctx, "documents")
if err != nil {
    return err
}
defer table.Close()

if qv2, ok := table.(contracts.ITableQueryV2); ok {
    record, err := qv2.QueryV2().Limit(10).Execute(ctx)
    if err != nil {
        return err
    }
    defer record.Release()
    // use record
}
```

Every concrete table implements all companion interfaces in the preview line;
the assertion still keeps callers compatible with an older or external
implementation.

## Connections and schemas

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-quickstart-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("text", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "documents", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"hello", "world"}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	count, err := table.Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows:", count)

	rows, err := table.SelectWithFilter(ctx, "id = 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("id = 1:", rows)
}
```

`Connect` keeps the published `ConnectionOptions` shape. `Region` and
`ReadConsistencyInterval` are preserved fields but remain ignored, as in
v0.1.2. The structured S3 path reads only access key, secret key, session
token, region and profile; other fields and the GCS/Azure/Local structs are
not applied by `Connect`. Use `ConnectWithStorageOptions` for any option the
published structs cannot express.

`ConnectionOptions.StorageOptions` and the storage key constants are described
in [Cloud storage](#cloud-storage). `ISchemaBuilder` methods all return the
builder, and `Build` returns `(contracts.ISchema, error)`. `IsSchema.Fields`
returns `[]arrow.Field`, `ToArrowSchema` returns the equivalent
`*arrow.Schema`, and `NewSchema` wraps an existing `*arrow.Schema` without
copying it.

Connection and table handles are safe for concurrent use; query builders are
not. `Close` is idempotent and safe to call twice. The next sections record
what stays valid after a close.

## Queries

The published path returns every result batch as a slice of `arrow.Record` and
keeps v0.1.2 semantics. The additive path takes a context, concatenates the
batches and returns one `arrow.Record`; the caller owns and releases it.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-queries-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("text", false).
		AddInt32Field("score", false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3, 4, 5}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"alpha", "beta", "gamma", "delta", "epsilon"}, nil)
	builder.Field(2).(*array.Int32Builder).AppendValues([]int32{10, 20, 30, 40, 50}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	// The additive builder returns one concatenated record. The caller owns it.
	qv2 := table.(contracts.ITableQueryV2)
	result, err := qv2.QueryV2().
		Columns([]string{"id", "text"}).
		Filter("score >= 30").
		WithRowID().
		Limit(2).
		Execute(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer result.Release()

	fmt.Println("query columns:", result.Schema())
	ids := result.Column(0).(*array.Int32)
	texts := result.Column(1).(*array.String)
	for i := 0; i < int(result.NumRows()); i++ {
		fmt.Println("row:", ids.Value(i), texts.Value(i))
	}

	// SelectV2 is the map-row form of the same additive query path.
	limit := 1
	rows, err := qv2.SelectV2(ctx, contracts.QueryConfigV2{
		Columns: []string{"id", "text"},
		Where:   "text = 'beta'",
		Limit:   &limit,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("select v2:", rows)

	// The published v0.1.2 path still returns one map per row; numeric values
	// are JSON numbers, so integral columns decode as float64.
	legacy, err := table.SelectWithLimit(ctx, 2, 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("legacy offset 3:", legacy)
}
```

Details that matter:

- `IQueryBuilder.Execute()` calls the native query with `context.Background()`
  and returns `[]arrow.Record`; each returned record must be released. It does
  not observe the caller's context, matching v0.1.2.
- `IQueryBuilderV2.Execute(ctx)` checks `ctx.Err()` before creating the native
  query, then returns one record. `ExecuteAsync(ctx)` runs it in a goroutine
  and delivers the record or error on buffered channels, then closes both.
- `SelectV2` returns map rows decoded from the same result. `SelectIPCV2`
  returns the raw Arrow IPC file bytes for callers that need the exact Arrow
  schema and zero conversion loss.
- `QueryV2().Limit(0)` leaves the native limit unset and returns all matching
  rows; a legacy `QueryConfig.Limit` pointing at zero returns zero rows. A
  negative legacy limit is ignored (unbounded), while a negative `QueryV2`
  limit is an error. A vector query requires a positive `Limit`.
- `WithRowID` adds `_rowid` (`uint64`). `FastSearch` skips rows not yet
  covered by an index. `ApplyOptions` maps `QueryOptions.MaxResults` to
  `Limit` when it is positive.
- Map rows are convenience values, not a lossless Arrow representation.
  Int32/Int64 become `float64`, booleans stay `bool`, strings stay `string`,
  and nulls become `nil`. The legacy map path turns Int8/Int16, UInt8, Binary,
  timestamp and variable-length List values into `"Unsupported type: ..."`
  strings; the V2 map path also decodes unsigned integers and List values.
  Fixed-size lists decode recursively, and non-finite floats become `nil`. Use
  `QueryV2`/`SelectIPCV2` when the exact Arrow type matters.

Maintained coverage: [`examples/basic_crud`](../examples/basic_crud/),
`pkg/tests/query_builder_test.go`, `pkg/tests/legacy_query_compat_test.go`.

## Vector search

A vector query needs a non-empty column and a positive `Limit`. The distance
metric is optional; omitting it is distinct from choosing L2.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const dim = 4

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-vector-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddVectorField("embedding", dim, contracts.VectorDataTypeFloat32, false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "vectors", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	arrowSchema := schema.ToArrowSchema()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowSchema)
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3, 4}, nil)
	embeddings := builder.Field(1).(*array.FixedSizeListBuilder)
	values := embeddings.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < 4; i++ {
		embeddings.Append(true)
		values.AppendValues([]float32{float32(i), float32(i) + 0.1, 0, 1}, nil)
	}
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	query := []float32{1, 1.1, 0, 1}
	result, err := table.(contracts.ITableVectorQuery).
		VectorQuery("embedding", query).
		DistanceType(contracts.DistanceTypeCosine).
		BypassVectorIndex().
		Columns([]string{"id"}).
		WithRowID().
		Limit(3).
		Execute(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer result.Release()

	fmt.Println("vector result:", result.Schema())
	ids := result.Column(0).(*array.Int32)
	for i := 0; i < int(result.NumRows()); i++ {
		fmt.Println("id:", ids.Value(i))
	}

	// The published API keeps its map-row vector search.
	nearest, err := table.VectorSearch(ctx, "embedding", query, 2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("legacy nearest:", nearest)
}
```

- `IVectorQueryBuilderV2` methods: `Filter`, `Limit`, `Columns`,
  `DistanceType`, `Nprobes`, `RefineFactor`, `Ef`, `BypassVectorIndex`,
  `WithRowID`, `FastSearch`, `Postfilter`, `Rerank`, `WithFullText`,
  `Execute`, `ExecuteAsync`, `ApplyOptions`.
- `Limit(k)` is the top-k count. `DistanceType` accepts
  `DistanceTypeL2`, `DistanceTypeCosine`, `DistanceTypeDot` or
  `DistanceTypeHamming` and only affects the builder when set.
- `Nprobes`, `RefineFactor` and `Ef` tune index search; `BypassVectorIndex`
  forces a scan. `Postfilter` evaluates the filter after building the
  candidate set and is only valid on a vector query.
- Vector results include `_distance` (`float32`). `WithRowID` adds `_rowid`.
  `Columns` selects the payload columns; the SDK keeps the search columns it
  needs.
- The published `ITable.VectorSearch` and `VectorSearchWithFilter` still
  return map rows and are implemented through the published query path.

Maintained coverage: [`examples/vector_search`](../examples/vector_search/),
[`examples/query_tuning`](../examples/query_tuning/),
`pkg/tests/query_tuning_test.go`.

## Full-text and hybrid search

Full-text search in the additive API takes either an `FTSSearch` in
`QueryConfigV2` or `WithFullText` on a vector query. Create an FTS index
before querying; hybrid search needs the same index.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const dim = 4

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-text-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("body", false).
		AddVectorField("embedding", dim, contracts.VectorDataTypeFloat32, false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "docs", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	bodies := []string{
		"the quick brown fox jumps over the lazy dog",
		"a slow blue cat sleeps on the warm mat",
		"the red fox hunts under the moonlit night",
		"lancedb makes fast vector search feel ordinary",
	}
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3, 4}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues(bodies, nil)
	embeddings := builder.Field(2).(*array.FixedSizeListBuilder)
	values := embeddings.ValueBuilder().(*array.Float32Builder)
	for i := range bodies {
		embeddings.Append(true)
		values.AppendValues([]float32{float32(i), 1, 0, 0}, nil)
	}
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	// A full-text index is required before FTS and hybrid search can use it.
	indexes := table.(contracts.ITableIndexManagement)
	if err := indexes.CreateIndexWithParams(ctx,
		[]string{"body"},
		contracts.IndexTypeFts,
		contracts.IndexParams{FtsLanguage: "English"},
		&contracts.CreateIndexOptions{Name: "body_fts", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatal(err)
	}

	qv2 := table.(contracts.ITableQueryV2)
	rows, err := qv2.SelectV2(ctx, contracts.QueryConfigV2{
		Columns:   []string{"id", "body"},
		FTSSearch: &contracts.FTSSearch{Column: "body", Query: "fox"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("fts:", rows)

	query := []float32{1, 1, 0, 0}
	hybrid, err := table.(contracts.ITableVectorQuery).
		VectorQuery("embedding", query).
		WithFullText("fox", "body").
		Rerank(contracts.RerankerConfig{
			Kind: contracts.RerankerRRF,
			RRFK: 60,
			Norm: contracts.NormalizeRank,
		}).
		Columns([]string{"id", "body"}).
		Limit(3).
		Execute(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer hybrid.Release()

	fmt.Println("hybrid:", hybrid.Schema())
	ids := hybrid.Column(0).(*array.Int32)
	for i := 0; i < int(hybrid.NumRows()); i++ {
		fmt.Println("hit:", ids.Value(i))
	}
}
```

- FTS results use `_score` (`float32`). Hybrid results use
  `_relevance_score` and default to rank fusion; an explicit
  `RerankerConfig{Kind: RerankerRRF, RRFK: 60, Norm: NormalizeRank}` sets the
  RRF constant and normalization.
- `RerankerNone` disables reranking. A reranker or `Postfilter` on a
  `QueryConfigV2` without a vector query is rejected.
- The legacy `FullTextSearch` and `FullTextSearchWithFilter` keep the v0.1.2
  `Full-text search is not currently supported` error. Use `SelectV2` or the
  additive builders.
- Hybrid search requires a text index; vector-only search does not.

Maintained coverage:
[`examples/hybrid_vector_fts`](../examples/hybrid_vector_fts/),
[`examples/hybrid_search`](../examples/hybrid_search/),
[`examples/reranker`](../examples/reranker/),
`pkg/tests/fts_test.go`, `pkg/tests/hybrid_search_test.go`.

## Mutations

`ITable.Update` keeps its v0.1.2 map form; `ITableUpdateExpr.UpdateExpr` is the
raw-SQL form with a result. Both use the same SQL dialect as queries.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-mutations-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("s", false).
		AddInt32Field("score", false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"active", "inactive", "active"}, nil)
	builder.Field(2).(*array.Int32Builder).AppendValues([]int32{10, 20, 30}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	// Update takes JSON scalar values. Strings become SQL single-quoted
	// literals; composite values are rejected before any write.
	if err := table.Update(ctx, "s = 'active'", map[string]interface{}{"score": 99}); err != nil {
		log.Fatal(err)
	}
	if err := table.Update(ctx, "id = 2", map[string]interface{}{"s": "archived"}); err != nil {
		log.Fatal(err)
	}
	if err := table.Update(ctx, "id = 1", map[string]interface{}{"score": []string{"composite"}}); err == nil {
		log.Fatal("composite update value unexpectedly succeeded")
	}

	// UpdateExpr forwards raw SQL expressions and reports affected rows and
	// the committed version.
	expr := table.(contracts.ITableUpdateExpr)
	result, err := expr.UpdateExpr(ctx, "s = 'active'", []contracts.UpdateAssignment{
		{Column: "score", Expr: "score + 1"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("update expr: rows=%d version=%d\n", result.RowsUpdated, result.Version)

	if err := table.Delete(ctx, "id = 2"); err != nil {
		log.Fatal(err)
	}

	rows, err := table.SelectWithColumns(ctx, []string{"id", "s", "score"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows:", rows)
}
```

- `Update` rejects an empty predicate and an empty update map. It serializes
  each value with `json.Marshal` first: strings become single-quoted literals
  with embedded single quotes doubled, numbers and booleans pass through,
  `null` becomes `NULL`, and composite or non-JSON values fail before any
  write. `time.Time` and custom `json.Marshaler` values therefore follow their
  JSON representation.
- `UpdateExpr` takes `[]UpdateAssignment{Column, Expr}`. Each `Expr` is
  forwarded verbatim; the caller writes the SQL. It permits an empty filter
  for a whole-table update and returns `RowsUpdated` and the committed
  `Version`.
- `Delete` forwards its predicate and returns only an error. All three methods
  reject embedded NUL bytes in predicates, column names and expressions,
  because a C string would truncate them.
- Mutations are in-place commits on the table handle. Read the new state with
  `Count`, `Version` or a query; use time travel to inspect older versions.

Maintained coverage: [`examples/basic_crud`](../examples/basic_crud/),
`pkg/tests/legacy_update_test.go`, `pkg/tests/update_expr_test.go`,
`pkg/tests/sql_dialect_test.go`.

## Merge insert

Merge insert compares a source record set with the target table on one or more
key columns and applies the configured actions in one atomic operation.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func record(schema *arrow.Schema, ids []int32, names []string, scores []int32) arrow.Record {
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues(ids, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues(names, nil)
	builder.Field(2).(*array.Int32Builder).AppendValues(scores, nil)
	return builder.NewRecord()
}

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-merge-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("name", false).
		AddInt32Field("score", false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "users", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	arrowSchema := schema.ToArrowSchema()
	seed := record(arrowSchema, []int32{1, 2, 3}, []string{"alice", "bob", "carol"}, []int32{50, 50, 50})
	defer seed.Release()
	if err := table.Add(ctx, seed, nil); err != nil {
		log.Fatal(err)
	}

	merge := table.(contracts.ITableMergeInsert)
	upsert := record(arrowSchema, []int32{2, 3, 4}, []string{"bob-v2", "carol-v2", "dave"}, []int32{200, 300, 400})
	defer upsert.Release()
	result, err := merge.MergeInsert([]string{"id"}).
		WhenMatchedUpdateAll(nil).
		WhenNotMatchedInsertAll().
		UseIndex(true).
		Timeout(30*time.Second).
		Execute(ctx, []arrow.Record{upsert})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("upsert: inserted=%d updated=%d deleted=%d version=%d\n",
		result.NumInsertedRows, result.NumUpdatedRows, result.NumDeletedRows, result.Version)

	// target.* and source.* reference the existing and incoming rows.
	condition := "target.score < source.score"
	candidate := record(arrowSchema, []int32{1, 2}, []string{"alice-new", "bob-new"}, []int32{100, 10})
	defer candidate.Release()
	result, err = merge.MergeInsert([]string{"id"}).
		WhenMatchedUpdateAll(&condition).
		Execute(ctx, []arrow.Record{candidate})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("conditional update: updated=%d\n", result.NumUpdatedRows)

	rows, err := table.SelectWithColumns(ctx, []string{"id", "name", "score"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows:", rows)
}
```

- `MergeInsert(on []string)` returns an `IMergeInsertBuilder`. At least one
  action must be configured: `WhenMatchedUpdateAll(condition *string)`,
  `WhenNotMatchedInsertAll()` or `WhenNotMatchedBySourceDelete(filter
  *string)`.
- Conditions reference `target.*` for the existing row and `source.*` for the
  incoming row. A nil condition means "all matched rows".
- `Timeout(d)` bounds the whole merge including retries. `UseIndex(bool)`
  toggles the join-key index, whose server-side default is true.
- `Execute(ctx, records)` borrows the records, serializes them to Arrow IPC
  and returns `*MergeResult{Version, NumInsertedRows, NumUpdatedRows,
  NumDeletedRows, NumAttempts}`. An empty record slice uses the table schema
  as an empty source.
- The merge keys and conditions reject embedded NUL bytes. The builder is not
  reusable across concurrent calls.

Maintained coverage:
[`examples/merge_insert`](../examples/merge_insert/),
`pkg/tests/merge_insert_test.go`.

## Schema evolution

Schema evolution commits a new version and returns it as a `uint64`. The table
handle continues to refer to the current (new) version.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-schema-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("name", false).
		AddInt32Field("score", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"alice", "bob", "carol"}, nil)
	builder.Field(2).(*array.Int32Builder).AppendValues([]int32{10, 20, 30}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	evolve := table.(contracts.ITableSchemaEvolve)

	version, err := evolve.AddColumns(ctx, []contracts.NewColumnTransform{
		{Name: "score_x2", Expression: "score * 2"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("after AddColumns, version:", version)

	title := "title"
	nullable := true
	version, err = evolve.AlterColumns(ctx, []contracts.ColumnAlteration{
		{Path: "name", Rename: &title},
		{Path: "id", Nullable: &nullable},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("after AlterColumns, version:", version)

	version, err = evolve.DropColumns(ctx, []string{"score_x2"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("after DropColumns, version:", version)

	current, err := table.Schema(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, field := range current.Fields() {
		fmt.Printf("field: %s nullable=%t type=%s\n", field.Name, field.Nullable, field.Type)
	}

	rows, err := table.SelectWithColumns(ctx, []string{"id", "title", "score"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows:", rows)
}
```

- `AddColumns` takes `[]NewColumnTransform{Name, Expression}` and evaluates
  each SQL expression against every existing row. It rejects an empty slice,
  an empty name and an empty expression before the native call; duplicate or
  malformed columns are native errors.
- `AlterColumns` takes `[]ColumnAlteration{Path, Rename, Nullable}`. Each
  alteration needs at least one of `Rename` or `Nullable`. `Rename` changes the
  field name; `Nullable` widens or narrows nullability subject to the backend's
  rules (the pinned backend allows widening, not tightening).
- `DropColumns` takes the field names to remove.
- `Table.Schema(ctx)` returns the current Arrow schema; inspect the field
  names, types and nullability after a change.

Maintained coverage: `pkg/tests/schema_evolve_test.go`.

## Time travel

Versions are strictly increasing commits. `Checkout` pins a table handle to a
version; reads see that version and writes fail until the pin is released.
`Restore` promotes the checked-out version into a new latest commit.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func record(schema *arrow.Schema, ids []int32, names []string) arrow.Record {
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues(ids, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues(names, nil)
	return builder.NewRecord()
}

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-timetravel-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("name", false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	arrowSchema := schema.ToArrowSchema()
	first := record(arrowSchema, []int32{1, 2}, []string{"alice", "bob"})
	defer first.Release()
	if err := table.Add(ctx, first, nil); err != nil {
		log.Fatal(err)
	}
	second := record(arrowSchema, []int32{3}, []string{"carol"})
	defer second.Release()
	if err := table.Add(ctx, second, nil); err != nil {
		log.Fatal(err)
	}

	travel := table.(contracts.ITableTimeTravel)
	versions, err := travel.ListVersions(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, version := range versions {
		fmt.Printf("version: %d at %s\n", version.Version, version.Timestamp.Format("2006-01-02T15:04:05Z"))
	}

	// Checkout pins this table handle to an older version. Reads see that
	// version; writes are rejected until the pin is released.
	if err := travel.Checkout(ctx, 2); err != nil {
		log.Fatal(err)
	}
	count, err := table.Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows at version 2:", count)
	pinned := record(arrowSchema, []int32{99}, []string{"blocked"})
	defer pinned.Release()
	if err := table.Add(ctx, pinned, nil); err == nil {
		log.Fatal("write unexpectedly succeeded on a checked-out table")
	} else {
		fmt.Println("write while checked out:", err)
	}

	// Restore promotes the checked-out version into a new latest commit and
	// releases the pin; history grows by one version.
	if err := travel.Restore(ctx); err != nil {
		log.Fatal(err)
	}
	count, err = table.Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows after Restore:", count)

	if err := travel.CheckoutLatest(ctx); err != nil {
		log.Fatal(err)
	}

	// Tags are metadata and can be managed while a handle is pinned.
	if err := travel.TagCreate(ctx, "release-2", 2); err != nil {
		log.Fatal(err)
	}
	tagVersion, err := travel.TagGetVersion(ctx, "release-2")
	if err != nil {
		log.Fatal(err)
	}
	tags, err := travel.TagList(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tag version:", tagVersion, "tags:", len(tags))

	if err := travel.CheckoutTag(ctx, "release-2"); err != nil {
		log.Fatal(err)
	}
	count, err = table.Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("rows at tag release-2:", count)

	if err := travel.CheckoutLatest(ctx); err != nil {
		log.Fatal(err)
	}
	latest, err := table.Version(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := travel.TagUpdate(ctx, "release-2", uint64(latest)); err != nil {
		log.Fatal(err)
	}
	if err := travel.TagDelete(ctx, "release-2"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("latest version:", latest)
}
```

- `ListVersions` returns `[]VersionInfo{Version, Timestamp, Metadata}`;
  timestamps are UTC. `Version` returns the handle's current version as `int`.
- `Checkout(ctx, version)`, `CheckoutTag(ctx, tag)` and `CheckoutLatest(ctx)`
  move the pin. A write while pinned fails with the backend's "table cannot be
  modified when a specific version is checked out" error.
- `Restore` requires a checkout; it creates a new commit carrying the pinned
  version's data and releases the pin. It does not move or delete history.
- Tags are metadata, not dataset commits: `TagCreate`, `TagGetVersion`,
  `TagList`, `TagUpdate` and `TagDelete` work while a handle is pinned.
  `TagInfo` carries `Version`, `ManifestSize` and `Branch`.

Maintained coverage: `pkg/tests/time_travel_test.go`.

## Index management and optimization

Index creation, inspection, wait, prewarm and drop are separate capabilities
from the published `CreateIndex`/`CreateIndexWithName` methods.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const dim = 64

func u32(v uint32) *uint32 { return &v }
func u64(v uint64) *uint64 { return &v }
func distance(v contracts.DistanceType) *contracts.DistanceType {
	return &v
}

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-index-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("body", false).
		AddVectorField("embedding", dim, contracts.VectorDataTypeFloat32, false).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "docs", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	const rows = 300
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	ids := builder.Field(0).(*array.Int32Builder)
	bodies := builder.Field(1).(*array.StringBuilder)
	embeddings := builder.Field(2).(*array.FixedSizeListBuilder)
	values := embeddings.ValueBuilder().(*array.Float32Builder)
	for i := 0; i < rows; i++ {
		ids.Append(int32(i))
		bodies.Append(fmt.Sprintf("the quick brown fox jumps %d", i))
		embeddings.Append(true)
		for j := 0; j < dim; j++ {
			values.Append(float32(i)*0.01 + float32(j)*0.001)
		}
	}
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	indexes := table.(contracts.ITableIndexManagement)
	if err := table.CreateIndexWithName(ctx, []string{"id"}, contracts.IndexTypeBTree, "id_btree"); err != nil {
		log.Fatal(err)
	}
	if err := indexes.CreateIndexWithParams(ctx,
		[]string{"embedding"},
		contracts.IndexTypeIvfPq,
		contracts.IndexParams{
			NumPartitions: u32(8),
			NumSubVectors: u32(4),
			NumBits:       u32(8),
			DistanceType:  distance(contracts.DistanceTypeCosine),
		},
		&contracts.CreateIndexOptions{Name: "emb_ivfpq", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatal(err)
	}
	if err := indexes.CreateIndexWithParams(ctx,
		[]string{"body"},
		contracts.IndexTypeFts,
		contracts.IndexParams{FtsLanguage: "English"},
		&contracts.CreateIndexOptions{Name: "body_fts", WaitTimeout: 30 * time.Second},
	); err != nil {
		log.Fatal(err)
	}
	if err := indexes.WaitForIndex(ctx, []string{"emb_ivfpq", "body_fts"}, 30*time.Second); err != nil {
		log.Fatal(err)
	}

	listed, err := table.GetAllIndexes(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, index := range listed {
		fmt.Printf("index: %s columns=%v type=%s\n", index.Name, index.Columns, index.IndexType)
	}

	stats, err := indexes.IndexStats(ctx, "emb_ivfpq")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("stats: indexed=%d unindexed=%d type=%s\n",
		stats.NumIndexedRows, stats.NumUnindexedRows, stats.IndexType)

	if err := table.(contracts.ITablePrewarmIndex).PrewarmIndex(ctx, "emb_ivfpq"); err != nil {
		log.Fatal(err)
	}
	if err := indexes.DropIndex(ctx, "id_btree"); err != nil {
		log.Fatal(err)
	}

	optimize := table.(contracts.ITableOptimize)
	compacted, err := optimize.OptimizeWithAction(ctx, contracts.OptimizeAction{
		Kind: contracts.OptimizeCompact,
		Compaction: contracts.CompactionParams{
			TargetRowsPerFragment: u64(1024),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	pruned, err := optimize.OptimizeWithAction(ctx, contracts.OptimizeAction{
		Kind: contracts.OptimizePrune,
		Prune: contracts.PruneParams{
			OlderThan:        24 * time.Hour,
			DeleteUnverified: ptr(false),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("compact fragments removed=%d added=%d\n",
		value(compacted.Compaction.FragmentsRemoved), value(compacted.Compaction.FragmentsAdded))
	fmt.Printf("prune versions=%d bytes=%d\n",
		value(pruned.Prune.OldVersions), value(pruned.Prune.BytesRemoved))
}

func ptr(v bool) *bool { return &v }

func value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
```

- `CreateIndexWithParams(ctx, columns, indexType, params, opts)` accepts the
  full `IndexParams` tuning set and `CreateIndexOptions{Name, Replace,
  WaitTimeout}`. A zero `WaitTimeout` leaves the wait budget to the native
  default; a positive value bounds the creation wait.
- `CreateIndex` and `CreateIndexWithName` remain the v0.1.2 convenience
  methods and use backend defaults. `GetAllIndexes` reports the Go spellings
  (`BTree`, `IvfPq`, `FTS`, and so on).
- `IndexStats(ctx, name)` returns `*IndexStatistics` with indexed and
  unindexed row counts, type, optional distance type and optional index count.
  A missing native stats value is returned as `nil, nil`; check for nil before
  dereferencing.
- `WaitForIndex(ctx, names, timeout)` folds `timeout` and the context deadline
  into one budget. Zero means no upper bound; a negative timeout becomes a
  1 ms budget; a positive sub-millisecond budget floors to 1 ms. An empty name
  slice takes one index snapshot and waits for those names. The native SDK may
  reject a budget above its documented limit.
- `DropIndex(ctx, name)` and `PrewarmIndex(ctx, name)` operate by index name.
- `OptimizeWithAction` takes `OptimizeAction{Kind, Compaction, Prune}` and
  returns `OptimizeStats`. `OptimizeCompact` reports fragment and file
  add/remove counts; `OptimizePrune` reports removed bytes and old versions.
  Unset numeric fields are pointers, so omission stays distinct from zero.

Maintained coverage:
[`examples/index_management`](../examples/index_management/),
[`examples/index_builder`](../examples/index_builder/),
[`examples/optimize_action`](../examples/optimize_action/),
[`examples/wait_for_index`](../examples/wait_for_index/),
`pkg/tests/get_all_indexes_test.go`, `pkg/tests/optimize_action_test.go`.

## Cloud storage

`Connect` keeps the published structured options; `ConnectWithStorageOptions`
takes a flat `map[string]string` and is the additive path. Options are applied
to that connection only; the SDK never mutates process environment variables.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	// Local storage needs no options. The flat-map entry point is the additive
	// path for options the published ConnectionOptions structs cannot express.
	dir, err := os.MkdirTemp("", "lancedb-storage-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	db, err := lancedb.ConnectWithStorageOptions(ctx, dir, map[string]string{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("local connection:", !db.IsClosed())
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}

	// An S3-compatible fixture, for example MinIO on loopback. Credentials
	// come from the environment so this program contains no secrets.
	if uri := os.Getenv("LANCEDB_GUIDE_S3_URI"); uri != "" {
		region := os.Getenv("LANCEDB_GUIDE_S3_REGION")
		if region == "" {
			region = "<region>"
		}
		s3Options := map[string]string{
			contracts.StorageAWSAccessKeyID:     os.Getenv("LANCEDB_GUIDE_S3_ACCESS_KEY"),
			contracts.StorageAWSSecretAccessKey: os.Getenv("LANCEDB_GUIDE_S3_SECRET_KEY"),
			contracts.StorageAWSRegion:          region,
			contracts.StorageEndpoint:           os.Getenv("LANCEDB_GUIDE_S3_ENDPOINT"),
			contracts.StorageS3AddressingStyle:  "path",
			contracts.StorageAllowHTTP:          "true",
		}
		s3, err := lancedb.ConnectWithStorageOptions(ctx, uri, s3Options)
		if err != nil {
			log.Fatal(err)
		}
		defer s3.Close()

		schema, err := lancedb.NewSchemaBuilder().AddInt32Field("id", false).Build()
		if err != nil {
			log.Fatal(err)
		}
		name := fmt.Sprintf("storage_check_%d", time.Now().UnixNano())
		table, err := s3.CreateTable(ctx, name, schema)
		if err != nil {
			log.Fatal(err)
		}
		if err := table.Close(); err != nil {
			log.Fatal(err)
		}
		if err := s3.DropTable(ctx, name); err != nil {
			log.Fatal(err)
		}
		fmt.Println("s3 fixture: create and drop succeeded")
	} else {
		fmt.Println("s3 fixture skipped: set LANCEDB_GUIDE_S3_URI, LANCEDB_GUIDE_S3_ENDPOINT,")
		fmt.Println("LANCEDB_GUIDE_S3_ACCESS_KEY and LANCEDB_GUIDE_S3_SECRET_KEY to run it")
	}

	fmt.Println("S3:", map[string]string{
		contracts.StorageAWSAccessKeyID:     "<access-key>",
		contracts.StorageAWSSecretAccessKey: "<secret-key>",
		contracts.StorageAWSRegion:          "<region>",
	})
	fmt.Println("MinIO:", map[string]string{
		contracts.StorageEndpoint:          "http://127.0.0.1:9000",
		contracts.StorageS3AddressingStyle: "path",
		contracts.StorageAllowHTTP:         "true",
	})
	fmt.Println("R2:", map[string]string{
		contracts.StorageAWSEndpoint: "https://<account-id>.r2.cloudflarestorage.com",
		contracts.StorageAWSRegion:   "auto",
	})
	fmt.Println("GCS:", map[string]string{
		contracts.StorageGCSServiceAccount: "<service-account.json>",
	})
	fmt.Println("Azure:", map[string]string{
		contracts.StorageAzureAccountName: "<account>",
		contracts.StorageAzureAccessKey:   "<key>",
	})
}
```

The published struct path, as a fragment with its `*string` helper:

```go
strPtr := func(s string) *string { return &s }
// Only these S3Config fields are read by Connect.
opts := &contracts.ConnectionOptions{
    StorageOptions: &contracts.StorageOptions{
        S3Config: &contracts.S3Config{
            AccessKeyID:     strPtr("<access-key>"),
            SecretAccessKey: strPtr("<secret-key>"),
            Region:          strPtr("<region>"),
            Profile:         strPtr("<profile>"),
        },
    },
}
db, err := lancedb.Connect(ctx, "s3://bucket/prefix", opts)

// Additive shape: arbitrary keys, per connection.
db, err := lancedb.ConnectWithStorageOptions(ctx, "s3://bucket/prefix", map[string]string{
    contracts.StorageAWSAccessKeyID:     "<access-key>",
    contracts.StorageAWSSecretAccessKey: "<secret-key>",
    contracts.StorageAWSRegion:          "<region>",
})
```

Key groups in `pkg/contracts/storage_keys.go`:

- **S3 / S3-compatible:** `StorageAWSAccessKeyID`, `StorageAWSSecretAccessKey`,
  `StorageAWSSessionToken`, `StorageAWSRegion`, `StorageEndpoint`,
  `StorageAWSEndpoint`, `StorageS3AddressingStyle`,
  `StorageVirtualHostedStyleRequest`, `StorageAllowHTTP`,
  `StorageSkipSignature`, `StorageRoleArn` and the rest of the `Storage*`
  constants in that file.
- **Native AWS provider:** `StorageAWSProfile` selects a named profile for
  embedded S3 and the DynamoDB commit handler of `s3+ddb://`.
  `StorageAWSRegion` wins over the environment and the profile.
  `StorageDynamoDBEndpoint` (or `DYNAMODB_ENDPOINT`) selects the DynamoDB
  endpoint independently.
- **R2:** use `StorageAWSEndpoint`, not `StorageEndpoint`, for
  `https://<account-id>.r2.cloudflarestorage.com`; region is `auto`.
- **GCS:** `StorageGCSServiceAccount` (path) or
  `StorageGCSServiceAccountKey`; `StorageGCSApplicationCredentials` names
  `GOOGLE_APPLICATION_CREDENTIALS`.
- **Azure:** `StorageAzureAccountName` with `StorageAzureAccessKey`,
  `StorageAzureSASToken`, or the tenant/client/secret trio.
- **Local:** no options are required. `Connect(ctx, dir, nil)` uses the local
  file system.

For a loopback fixture, create the bucket first, then pass
`StorageEndpoint=http://127.0.0.1:9000`,
`StorageS3AddressingStyle=path` and `StorageAllowHTTP=true` for MinIO, or the
equivalent endpoint for another S3-compatible service. Bucket existence,
credentials, network access and IAM permissions are the caller's
prerequisites; the SDK reports the native error if they are missing.

Maintained coverage:
[`examples/storage_configuration`](../examples/storage_configuration/),
`pkg/tests/storage_options_test.go`, `pkg/lancedb/aws_integration_test.go`.

## SQL dialect

The preview adopts the LanceDB SDK SQL dialect. This is a semantic change from
v0.1.2, not an error the SDK can detect:

| Predicate | Meaning |
| --- | --- |
| `s = 'active'` | Compare column `s` with the string literal `active`. |
| `s = "active"` | Compare column `s` with the column named `active`. |

The fixture in `pkg/tests/sql_dialect_test.go` uses a table with columns `id`,
`s` and `active` and verifies the selected and affected ids for query, update
and delete. The same text reaches the engine from `QueryConfig.Where`,
`QueryConfigV2.Where`, `IQueryBuilder.Filter`, `IVectorQueryBuilderV2.Filter`,
`Delete`, `Update`, `UpdateExpr` and merge-insert conditions, so review every
stored predicate that uses double quotes.

Predicates are DataFusion SQL. The SDK forwards them and returns the engine's
error; it does not implement a second parser, translate identifiers or escape
a caller's expression. Use single quotes for literals and double quotes for
identifiers. See [MIGRATION.md](../MIGRATION.md) for the full v0.1.2 behavior
comparison.

## Arrow ownership and lifetime

The IPC boundary is Arrow, not JSON. Schemas, input records and query results
cross it as Arrow IPC files, and all public values use Arrow Go v17.

**Input records are borrowed.** `Add`, `AddRecords` and `MergeInsert.Execute`
serialize the records to an IPC file before returning. The caller keeps
ownership and must call `Release` exactly once, as with any Arrow Go value.
Releasing immediately after the call returns is correct; releasing inside a
concurrent goroutine while the call is still running is not.

```go
record := builder.NewRecord()
defer record.Release()
if err := table.Add(ctx, record, nil); err != nil {
    return err
}
```

**Output records are owned by the caller.** The legacy `Execute` returns every
batch and the additive `Execute` returns one concatenated record; both are
decoded into Go-managed memory. Call `Release` when done. A result remains
valid after `Table.Close`, because it does not point into the table handle or
native memory.

**Schemas are values.** `ISchema` and `Table.Schema` return schema values that
do not need a release. `ISchema.ToArrowSchema()` returns the schema to use with
`array.NewRecordBuilder`.

**Builders require release too.** `array.NewRecordBuilder` returns a builder
with its own reference; call `builder.Release()` after `NewRecord()`. The
record is independent and keeps its own reference.

**Memory allocator.** The SDK uses the Go allocator for decoded results; there
is no native allocator to release. Passing an allocator to Arrow builders is
still the caller's choice (`memory.DefaultAllocator` and
`memory.NewGoAllocator` both work).

**Library lifetime.** The `lancedb-c` shared library is loaded at most once per
process and is never unloaded. `lancedb-c` owns native threads, thread-local
error state and handles that may outlive a table, so unloading it while those
may still run is not safe. In exchange, no `Close` on a connection or table
unloads the library.

**Handle lifetime.** `Table.Close` and `Connection.Close` are idempotent and
safe to call twice. A table closed directly rejects further operations with
`table is closed`. Closing the connection makes `IConnection` operations fail
with `connection is closed`; `ITable.IsOpen` becomes false, and the legacy
`Query().Execute()` path reports `table is closed`. Release result records
before your own exit; the GC finalizers are a fallback, not a substitute for
`Close`/`Release`.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-lifecycle-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("text", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}

	// Add serializes the record synchronously. The caller keeps ownership and
	// may release it as soon as Add returns.
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"a", "b", "c"}, nil)
	record := builder.NewRecord()
	builder.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}
	record.Release()

	// A decoded result owns Go memory. It stays valid after the table closes.
	result, err := table.(contracts.ITableQueryV2).QueryV2().Execute(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := table.Close(); err != nil {
		log.Fatal(err)
	}
	ids := result.Column(0).(*array.Int32)
	fmt.Println("result after table.Close, first id:", ids.Value(0))
	result.Release()

	fmt.Println("table open after Close:", table.IsOpen())
	if err := table.Close(); err != nil {
		log.Fatal(err)
	}
	if err := table.Add(ctx, nil, nil); err != nil {
		fmt.Println("Add on closed table:", err)
	}

	// Closing the connection invalidates its tables for the paths that check
	// the connection; closing a table afterwards remains safe.
	second, err := db.CreateTable(ctx, "second", schema)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("second table open after connection Close:", second.IsOpen())
	if _, err := second.Query().Execute(); err != nil {
		fmt.Println("legacy query after connection Close:", err)
	}
	if err := second.Close(); err != nil {
		log.Fatal(err)
	}
}
```

Maintained coverage: `pkg/tests/ipc_lifecycle_test.go`.

## Errors and context

Errors are ordinary Go `error` values. The SDK does not export error
sentinels or typed errors for native failures; the message contains the
operation and the native message. `context.Canceled` and
`context.DeadlineExceeded` are returned directly where context is observed,
so `errors.Is` works for those two.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "lancedb-errors-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := lancedb.Connect(ctx, dir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	schema, err := lancedb.NewSchemaBuilder().
		AddInt32Field("id", false).
		AddStringField("text", true).
		Build()
	if err != nil {
		log.Fatal(err)
	}
	table, err := db.CreateTable(ctx, "items", schema)
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema.ToArrowSchema())
	defer builder.Release()
	builder.Field(0).(*array.Int32Builder).AppendValues([]int32{1, 2, 3}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"a", "b", "c"}, nil)
	record := builder.NewRecord()
	defer record.Release()
	if err := table.Add(ctx, record, nil); err != nil {
		log.Fatal(err)
	}

	// Native and validation errors are ordinary Go errors. Nothing is
	// poisoned: a valid query works after a failed one.
	if _, err := table.SelectWithFilter(ctx, "this is not sql"); err != nil {
		fmt.Println("invalid SQL:", err)
	}
	rows, err := table.SelectWithFilter(ctx, "id = 1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("valid query after failure:", rows)

	// Additive query execution and WaitForIndex observe cancellation before
	// native work starts and return context.Canceled directly.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = table.(contracts.ITableQueryV2).QueryV2().Limit(1).Execute(canceled)
	fmt.Println("canceled query:", err, "is context.Canceled:", errors.Is(err, context.Canceled))
	err = table.(contracts.ITableIndexManagement).WaitForIndex(canceled, []string{"missing"}, time.Second)
	fmt.Println("canceled wait:", err, "is context.Canceled:", errors.Is(err, context.Canceled))

	// The published legacy path does not read ctx; it keeps v0.1.2 behavior.
	legacy, err := table.SelectWithFilter(canceled, "id = 2")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("legacy query with canceled ctx:", legacy)
}
```

Context is observed by `IQueryBuilderV2.Execute`/`ExecuteAsync`,
`IVectorQueryBuilderV2.Execute`/`ExecuteAsync`,
`ITableQueryV2.SelectV2`/`SelectIPCV2` and
`ITableIndexManagement.WaitForIndex`. These check cancellation before native
work starts; `WaitForIndex` also folds the deadline into its wait budget.
Legacy `Query().Execute()`, the map-row `Select` helpers, `Add`/`AddRecords`,
`Update`, `UpdateExpr`, `Delete`, `MergeInsert.Execute`, schema evolution and
time travel do not read the context and therefore keep executing with a
canceled context, as v0.1.2 did.

Cancellation is cooperative, not a rollback. Once a synchronous native write,
index build or merge has started, the SDK cannot interrupt it; a committed
mutation remains committed even if the caller cancels. Check `ctx.Err()`
before starting work and treat a returned `context.Canceled` as "stopped before
or between native calls", not "rolled back".

## Exported surface

### Package `lancedb`

```go
func Connect(ctx context.Context, uri string, options *contracts.ConnectionOptions) (contracts.IConnection, error)
func ConnectWithStorageOptions(ctx context.Context, uri string, storageOptions map[string]string) (contracts.IConnection, error)
func NewSchema(schema *arrow.Schema) (contracts.ISchema, error)
func NewSchemaBuilder() contracts.ISchemaBuilder
func RuntimeVersion() (string, error)
```

### Published interfaces

```go
type IConnection interface {
    Close() error
    TableNames(ctx context.Context) ([]string, error)
    OpenTable(ctx context.Context, name string) (ITable, error)
    CreateTable(ctx context.Context, name string, schema ISchema) (ITable, error)
    DropTable(ctx context.Context, name string) error
    IsClosed() bool
}

type ITable interface {
    Name() string
    IsOpen() bool
    Close() error
    Schema(ctx context.Context) (*arrow.Schema, error)
    Add(ctx context.Context, record arrow.Record, options *AddDataOptions) error
    AddRecords(ctx context.Context, records []arrow.Record, options *AddDataOptions) error
    Query() IQueryBuilder
    Count(ctx context.Context) (int64, error)
    Version(ctx context.Context) (int, error)
    Update(ctx context.Context, filter string, updates map[string]interface{}) error
    Delete(ctx context.Context, filter string) error
    CreateIndex(ctx context.Context, columns []string, indexType IndexType) error
    CreateIndexWithName(ctx context.Context, columns []string, indexType IndexType, name string) error
    GetAllIndexes(ctx context.Context) ([]IndexInfo, error)
    Select(ctx context.Context, config QueryConfig) ([]map[string]interface{}, error)
    SelectWithColumns(ctx context.Context, columns []string) ([]map[string]interface{}, error)
    SelectWithFilter(ctx context.Context, filter string) ([]map[string]interface{}, error)
    VectorSearch(ctx context.Context, column string, vector []float32, k int) ([]map[string]interface{}, error)
    VectorSearchWithFilter(ctx context.Context, column string, vector []float32, k int, filter string) ([]map[string]interface{}, error)
    FullTextSearch(ctx context.Context, column string, query string) ([]map[string]interface{}, error)
    FullTextSearchWithFilter(ctx context.Context, column string, query string, filter string) ([]map[string]interface{}, error)
    SelectWithLimit(ctx context.Context, limit int, offset int) ([]map[string]interface{}, error)
}

type IQueryBuilder interface {
    Filter(condition string) IQueryBuilder
    Limit(limit int) IQueryBuilder
    Execute() ([]arrow.Record, error)
    ExecuteAsync() (<-chan []arrow.Record, <-chan error)
    ApplyOptions(options *QueryOptions) IQueryBuilder
}

type IVectorQueryBuilder interface {
    Filter(condition string) IVectorQueryBuilder
    Limit(limit int) IVectorQueryBuilder
    DistanceType(_ DistanceType) IVectorQueryBuilder
    Execute() ([]arrow.Record, error)
    ExecuteAsync() (<-chan []arrow.Record, <-chan error)
    ApplyOptions(options *QueryOptions) IVectorQueryBuilder
}
```

`ISchema` and `ISchemaBuilder`:

```go
type ISchema interface {
    Fields() []arrow.Field
    NumFields() int
    Field(index int) (arrow.Field, error)
    FieldByName(name string) (arrow.Field, error)
    HasField(name string) bool
    String() string
    ToArrowSchema() *arrow.Schema
}

type ISchemaBuilder interface {
    AddField(name string, dataType arrow.DataType, nullable bool) ISchemaBuilder
    AddVectorField(name string, dimension int, dataType VectorDataType, nullable bool) ISchemaBuilder
    AddInt32Field(name string, nullable bool) ISchemaBuilder
    AddInt64Field(name string, nullable bool) ISchemaBuilder
    AddFloat32Field(name string, nullable bool) ISchemaBuilder
    AddFloat64Field(name string, nullable bool) ISchemaBuilder
    AddStringField(name string, nullable bool) ISchemaBuilder
    AddBinaryField(name string, nullable bool) ISchemaBuilder
    AddBooleanField(name string, nullable bool) ISchemaBuilder
    AddTimestampField(name string, unit arrow.TimeUnit, nullable bool) ISchemaBuilder
    Build() (ISchema, error)
}
```

### Companion interfaces

```go
type ITableQueryV2 interface {
    QueryV2() IQueryBuilderV2
    SelectV2(ctx context.Context, config QueryConfigV2) ([]map[string]interface{}, error)
    SelectIPCV2(ctx context.Context, config QueryConfigV2) ([]byte, error)
}

type IQueryBuilderV2 interface {
    Filter(condition string) IQueryBuilderV2
    Limit(limit int) IQueryBuilderV2
    Columns(columns []string) IQueryBuilderV2
    Offset(offset int) IQueryBuilderV2
    WithRowID() IQueryBuilderV2
    FastSearch() IQueryBuilderV2
    Execute(ctx context.Context) (arrow.Record, error)
    ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
    ApplyOptions(options *QueryOptions) IQueryBuilderV2
}

type ITableVectorQuery interface {
    VectorQuery(column string, vector []float32) IVectorQueryBuilderV2
}

type IVectorQueryBuilderV2 interface {
    Filter(condition string) IVectorQueryBuilderV2
    Limit(limit int) IVectorQueryBuilderV2
    Columns(columns []string) IVectorQueryBuilderV2
    DistanceType(dt DistanceType) IVectorQueryBuilderV2
    Nprobes(n int) IVectorQueryBuilderV2
    RefineFactor(n uint32) IVectorQueryBuilderV2
    Ef(n int) IVectorQueryBuilderV2
    BypassVectorIndex() IVectorQueryBuilderV2
    WithRowID() IVectorQueryBuilderV2
    FastSearch() IVectorQueryBuilderV2
    Postfilter() IVectorQueryBuilderV2
    Rerank(cfg RerankerConfig) IVectorQueryBuilderV2
    WithFullText(query, column string) IVectorQueryBuilderV2
    Execute(ctx context.Context) (arrow.Record, error)
    ExecuteAsync(ctx context.Context) (<-chan arrow.Record, <-chan error)
    ApplyOptions(options *QueryOptions) IVectorQueryBuilderV2
}

type ITableIndexManagement interface {
    CreateIndexWithParams(ctx context.Context, columns []string, indexType IndexType, params IndexParams, opts *CreateIndexOptions) error
    DropIndex(ctx context.Context, name string) error
    IndexStats(ctx context.Context, indexName string) (*IndexStatistics, error)
    WaitForIndex(ctx context.Context, names []string, timeout time.Duration) error
}

type ITableMergeInsert interface {
    MergeInsert(on []string) IMergeInsertBuilder
}

type IMergeInsertBuilder interface {
    WhenMatchedUpdateAll(condition *string) IMergeInsertBuilder
    WhenNotMatchedInsertAll() IMergeInsertBuilder
    WhenNotMatchedBySourceDelete(filter *string) IMergeInsertBuilder
    Timeout(d time.Duration) IMergeInsertBuilder
    UseIndex(useIndex bool) IMergeInsertBuilder
    Execute(ctx context.Context, records []arrow.Record) (*MergeResult, error)
}

type ITableOptimize interface {
    Optimize(ctx context.Context) (*OptimizeStats, error)
    OptimizeWithAction(ctx context.Context, action OptimizeAction) (*OptimizeStats, error)
}

type ITableUpdateExpr interface {
    UpdateExpr(ctx context.Context, filter string, assignments []UpdateAssignment) (*UpdateResult, error)
}

type ITableTimeTravel interface {
    ListVersions(ctx context.Context) ([]VersionInfo, error)
    Checkout(ctx context.Context, version uint64) error
    CheckoutTag(ctx context.Context, tag string) error
    CheckoutLatest(ctx context.Context) error
    Restore(ctx context.Context) error
    TagList(ctx context.Context) (map[string]TagInfo, error)
    TagGetVersion(ctx context.Context, tag string) (uint64, error)
    TagCreate(ctx context.Context, tag string, version uint64) error
    TagDelete(ctx context.Context, tag string) error
    TagUpdate(ctx context.Context, tag string, version uint64) error
}

type ITableSchemaEvolve interface {
    AddColumns(ctx context.Context, transforms []NewColumnTransform) (uint64, error)
    AlterColumns(ctx context.Context, alterations []ColumnAlteration) (uint64, error)
    DropColumns(ctx context.Context, names []string) (uint64, error)
}

type ITablePrewarmIndex interface {
    PrewarmIndex(ctx context.Context, name string) error
}
```

### Options and result types

| Type | Fields |
| --- | --- |
| `ConnectionOptions` | `Region *string`, `ReadConsistencyInterval *int`, `StorageOptions *StorageOptions` |
| `QueryOptions` | `MaxResults int`, `UseFullPrecision bool`, `BypassVectorIndex bool` |
| `QueryConfig` | `Columns []string`, `Where string`, `Limit *int`, `Offset *int`, `VectorSearch *VectorSearch`, `FTSSearch *FTSSearch` |
| `VectorSearch` | `Column string`, `Vector []float32`, `K int` |
| `FTSSearch` | `Column string`, `Query string` |
| `AddDataOptions` | `Mode WriteMode` (`WriteModeAppend`, `WriteModeOverwrite`; writes are always append) |
| `QueryConfigV2` | `Columns`, `Where`, `Limit *int`, `Offset *int`, `VectorSearch *VectorSearchV2`, `FTSSearch *FTSSearch`, `WithRowID`, `FastSearch`, `Postfilter`, `Reranker *RerankerConfig` |
| `VectorSearchV2` | `Column`, `Vector`, `K`, `DistanceType *DistanceType`, `Nprobes *int`, `RefineFactor *uint32`, `Ef *int`, `BypassVectorIndex bool`, `FullTextQuery`, `FullTextColumn` |
| `CreateIndexOptions` | `Name string`, `Replace bool`, `WaitTimeout time.Duration` |
| `IndexStatistics` | `NumIndexedRows int64`, `NumUnindexedRows int64`, `IndexType string`, `DistanceType *string`, `NumIndices *int` |
| `IndexInfo` | `Name string`, `Columns []string`, `IndexType string` |
| `RerankerConfig` | `Kind RerankerKind`, `RRFK float32`, `Norm NormalizeMethod` |
| `MergeResult` | `Version`, `NumInsertedRows`, `NumUpdatedRows`, `NumDeletedRows uint64`, `NumAttempts uint32` |
| `UpdateResult` | `RowsUpdated uint64`, `Version uint64` |
| `UpdateAssignment` | `Column string`, `Expr string` |
| `VersionInfo` | `Version uint64`, `Timestamp time.Time`, `Metadata map[string]string` |
| `TagInfo` | `Version uint64`, `ManifestSize uint64`, `Branch string` |
| `NewColumnTransform` | `Name string`, `Expression string` |
| `ColumnAlteration` | `Path string`, `Rename *string`, `Nullable *bool` |
| `OptimizeAction` | `Kind OptimizeActionKind`, `Compaction CompactionParams`, `Prune PruneParams` |
| `OptimizeStats` | `Compaction *CompactionMetrics`, `Prune *RemovalStats` |

`IndexParams` carries pointers so an unset value stays distinct from zero. Its
fields are `NumPartitions`, `SampleRate`, `MaxIterations`,
`TargetPartitionSize`, `NumSubVectors`, `NumBits`, `M`, `EfConstruction`
(`*uint32`), `DistanceType (*DistanceType)` plus the FTS fields
`FtsLanguage string`, `FtsWithPosition`, `FtsStem`, `FtsRemoveStopWords`,
`FtsLowerCase`, `FtsASCIIFolding`, `FtsNgramPrefixOnly` (`*bool`),
`FtsBaseTokenizer string`, `FtsMaxTokenLength`, `FtsNgramMinLength`,
`FtsNgramMaxLength` (`*uint32`). The FTS fields only affect an
`IndexTypeFts` index; IVF/HNSW fields only affect their index types.

Constants:

```go
type IndexType int        // IndexTypeAuto, IndexTypeIvfPq, IndexTypeIvfFlat,
                          // IndexTypeHnswPq, IndexTypeHnswSq, IndexTypeBTree,
                          // IndexTypeBitmap, IndexTypeLabelList, IndexTypeFts
type DistanceType int     // DistanceTypeL2 = 0, DistanceTypeCosine = 1,
                          // DistanceTypeDot = 2, DistanceTypeHamming = 3
type VectorDataType int   // VectorDataTypeFloat16, VectorDataTypeFloat32, VectorDataTypeFloat64
type WriteMode int        // WriteModeAppend, WriteModeOverwrite
type RerankerKind int     // RerankerNone, RerankerRRF
type NormalizeMethod int  // NormalizeDefault, NormalizeScore, NormalizeRank
type OptimizeActionKind int // OptimizeAll, OptimizeCompact, OptimizePrune, OptimizeIndex
```

The complete storage key list is in `pkg/contracts/storage_keys.go`; the
constants are stable names for the native option strings. `StorageAllowHTTP`,
`StorageSkipSignature`, `StorageS3AddressingStyle` and the credential keys are
the ones most programs need.

## Further reading

- [README](../README.md) for installation, supported targets and the quick
  start.
- [examples/README.md](../examples/README.md) for the maintained programs.
- [MIGRATION.md](../MIGRATION.md) for the v0.1.2 compatibility boundary and the
  SQL dialect change.
- [RELEASING.md](../RELEASING.md) for the root-tag and native-artifact release
  sequence.
- [pkg.go.dev/github.com/lancedb/lancedb-go](https://pkg.go.dev/github.com/lancedb/lancedb-go)
  for the symbol reference of the version you build against.
