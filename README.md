# LanceDB Go SDK

Pure Go bindings for [LanceDB](https://github.com/lancedb/lancedb). The SDK
loads the published `lancedb-c` shared library at runtime, so consumers need no
Rust toolchain, no C compiler and no `CGO_ENABLED=1` build.

- Go 1.21 or newer.
- `CGO_ENABLED=0`.
- Linux amd64/arm64, Windows amd64/arm64 and macOS Apple Silicon.
  Intel macOS is not supported.
- Apache Arrow Go v17 types (`github.com/apache/arrow/go/v17`).

## Install

Preview releases are published on the
[releases page](https://github.com/lancedb/lancedb-go/releases). Choose a
preview from that page whose release notes specify module-backed offline native
loading, then use that same tag in both commands below (replacing
`vX.Y.Z-preview.N`):

```bash
go get github.com/lancedb/lancedb-go@vX.Y.Z-preview.N
```

`@latest` selects the highest release version when the module has one; only a
module with no release version falls back to a prerelease or a pseudo-version.
The stable `v0.1.2` release predates the pure-Go preview and does not contain
the native payload or the `lancedb-go-native` helper, so name a preview
explicitly.

The native library for the running target is delivered as a compiled-in
companion module (`go.lancedb.io/lancedb-go/artifacts/<target>`) that the root
module requires. The first `Connect` verifies and extracts its payload into a
private per-user cache; it never downloads anything at runtime, so the SDK
works offline once `go get` has resolved the modules. No source build happens
on the consumer machine.

To pre-seed the cache (for example in an offline image or CI), run the helper
from the same tag:

```bash
go run github.com/lancedb/lancedb-go/pkg/cmd/lancedb-go-native@vX.Y.Z-preview.N
```

Discovery can be controlled explicitly:

| Environment variable | Purpose |
| --- | --- |
| `LANCEDB_C_LIBRARY_PATH` | Use this shared library file. When set, it is the only candidate; a bad path fails instead of falling back. |
| `LANCEDB_C_CACHE_DIR` | Override the private cache root. Default: the platform user cache directory plus `lancedb-go/native`. |

The runtime version must match the C header the module was built against. For
diagnostics, `lancedb.RuntimeVersion()` returns the loaded library's reported
version. The loaded library is never unloaded: `lancedb-c` owns native threads
and handles that may outlive a table.

The published Linux libraries require **glibc 2.35 or newer** on both amd64 and
arm64. The 2.34 loader reports missing `GLIBC_2.35`; use a distribution with
2.35+ (for example Ubuntu 22.04 or later).

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/apache/arrow/go/v17/arrow"
    "github.com/apache/arrow/go/v17/arrow/array"
    "github.com/apache/arrow/go/v17/arrow/memory"
    "github.com/lancedb/lancedb-go/pkg/contracts"
    "github.com/lancedb/lancedb-go/pkg/lancedb"
)

func main() {
    ctx := context.Background()
    db, err := lancedb.Connect(ctx, "./data", nil)
    if err != nil { log.Fatal(err) }
    defer db.Close()

    schema, err := lancedb.NewSchemaBuilder().
        AddInt32Field("id", false).
        AddStringField("text", true).
        AddVectorField("embedding", 4, contracts.VectorDataTypeFloat32, false).
        Build()
    if err != nil { log.Fatal(err) }

    table, err := db.CreateTable(ctx, "documents", schema)
    if err != nil { log.Fatal(err) }
    defer table.Close()

    arrowSchema := schema.ToArrowSchema()
    builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowSchema)
    defer builder.Release()
    builder.Field(0).(*array.Int32Builder).Append(1)
    builder.Field(1).(*array.StringBuilder).Append("hello")
    builder.Field(2).(*array.FixedSizeListBuilder).Append(true)
    builder.Field(2).(*array.FixedSizeListBuilder).ValueBuilder().(*array.Float32Builder).
        AppendValues([]float32{0.1, 0.2, 0.3, 0.4}, nil)
    record := builder.NewRecord()
    defer record.Release()
    if err := table.Add(ctx, record, nil); err != nil { log.Fatal(err) }

    count, err := table.Count(ctx)
    if err != nil { log.Fatal(err) }
    fmt.Println("rows:", count)

    rows, err := table.SelectWithFilter(ctx, "id = 1")
    if err != nil { log.Fatal(err) }
    fmt.Println(rows)
}
```

## Query API

The published v0.1.2 interfaces are preserved exactly. In particular,
`IQueryBuilder.Execute()` returns `([]arrow.Record, error)` and
`IVectorQueryBuilder.Execute()` keeps that shape.

Additive query controls use companion interfaces with a separate receiver, so
Go's lack of return-type overloading does not break published callers:

```go
qv2 := table.(contracts.ITableQueryV2)
record, err := qv2.QueryV2().
    Filter("score > 0.8").
    Columns([]string{"id", "text"}).
    Limit(10).
    Execute(ctx)
```

`contracts.ITableVectorQuery` provides `VectorQuery`, and
`contracts.ITableIndexManagement`, `ITableMergeInsert`, `ITableOptimize`,
`ITableUpdateExpr`, `ITableTimeTravel`, `ITableSchemaEvolve` and
`ITablePrewarmIndex` expose the other additive capabilities.

See [docs/api-guide.md](docs/api-guide.md) for complete programs covering
queries, vector and full-text search, mutations, merge insert, schema
evolution, time travel, index management, cloud storage, errors and Arrow
ownership.

Arrow IPC is the boundary for schemas, input records and query results. Input
batches are sent in one write; query results preserve every batch for the
published slice API and are concatenated for the context/single-record API.

## Cloud storage

`Connect` keeps the published v0.1.2 `ConnectionOptions` shape. Use
`ConnectWithStorageOptions` for the additive flat option map:

```go
db, err := lancedb.ConnectWithStorageOptions(ctx, "s3://bucket/prefix", map[string]string{
    contracts.StorageAWSProfile:        "primary",
    contracts.StorageAWSRegion:         "us-east-1",
    contracts.StorageEndpoint:          "http://127.0.0.1:9000",
    contracts.StorageAllowHTTP:         "true",
    contracts.StorageS3AddressingStyle: "path",
})
```

`aws_profile` selects a named profile for embedded S3 and for the DynamoDB
commit handler of `s3+ddb://`. `aws_region` wins over the environment and the
profile. `dynamodb_endpoint` (or `DYNAMODB_ENDPOINT`) chooses the DynamoDB
endpoint independently. The profile is applied per connection; the SDK does not
mutate process environment variables.

## SQL predicates

The migration adopts the LanceDB SDK SQL dialect. Double quotes delimit
column identifiers; single quotes delimit string literals. An existing update or
delete can therefore succeed while selecting different rows. Read
[MIGRATION.md](MIGRATION.md) before upgrading.

## Development

```bash
make test-native       # loader, cache and ABI layout checks
make test              # full suite (uses the compiled-in native payload)
make test-integration  # authenticated S3 + AWS profile/DynamoDB loopback tests
make vet fmt-check
```

See [RELEASING.md](RELEASING.md) for the root-module and native-artifact release
flow.
