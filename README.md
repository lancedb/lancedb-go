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

```bash
go get github.com/lancedb/lancedb-go
```

The first `Connect` downloads the pinned `lancedb-c` release for the running
target, verifies both the archive and extracted library SHA-256 digests, and
loads it from a private per-user cache. No source build happens on the consumer
machine.

To pre-seed the cache (for example in an offline image or CI):

```bash
go run github.com/lancedb/lancedb-go/cmd/lancedb-go-native@latest
```

Discovery can be controlled explicitly:

| Environment variable | Purpose |
| --- | --- |
| `LANCEDB_C_LIBRARY_PATH` | Use this shared library file. When set, it is the only candidate; a bad path fails instead of falling back. |
| `LANCEDB_C_CACHE_DIR` | Override the private cache root. Default: the platform user cache directory plus `lancedb-go/native`. |
| `LANCEDB_C_RELEASE_BASE_URL` | Use a release mirror or loopback fixture. The pinned asset names and digests still apply. |

The runtime version must match the C header the module was built against. The
loaded library is never unloaded: `lancedb-c` owns native threads and handles
that may outlive a table.

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

The migration adopts the current LanceDB SDK SQL dialect. Double quotes delimit
column identifiers; single quotes delimit string literals. An existing update or
delete can therefore succeed while selecting different rows. Read
[MIGRATION.md](MIGRATION.md) before upgrading.

## Development

```bash
make test-native       # loader, cache and ABI layout checks
make test              # full suite (downloads the pinned native artifact)
make test-integration  # authenticated S3 + AWS profile/DynamoDB loopback tests
make vet fmt-check
```

See [RELEASE_PROCESS.md](RELEASE_PROCESS.md) for the Go-module release flow.
