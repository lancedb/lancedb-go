# Migrating from v0.1.2 to the pure-Go preview

## Read this first: SQL predicates can select different rows

The migrated binding adopts the LanceDB SDK SQL dialect. The old binding and
the SDK do not interpret the same predicate text the same way:

- Single quotes delimit a **string literal**: `s = 'active'`.
- Double quotes delimit a **column identifier**: `s = "active"` compares the
  column `s` with the column named `active`.

An existing `Update` or `Delete` can therefore succeed while affecting a
different set of rows. This is a semantic change, not an error the SDK can
report. Review every stored predicate that uses double quotes and decide which
meaning was intended.

The migration verifies both forms through query, update and delete on a real
`CGO_ENABLED=0` consumer with a table containing a column named `active`:

| Predicate | Meaning | Rows selected/affected in the fixture |
| --- | --- | --- |
| `s = 'active'` | compare `s` to the literal `active` | ids 1 and 3 |
| `s = "active"` | compare `s` to the `active` column | id 1 |

The fixture is `pkg/tests/sql_dialect_test.go`. It checks the actual selected
and affected ids rather than only the absence of an error.

## Compatibility baseline

The published Go API is `v0.1.2` (`ba58a1c`). The migration keeps:

- `ITable`, `IQueryBuilder`, `IVectorQueryBuilder`, `IConnection`,
  `ISchema`, `ISchemaBuilder` and their published method sets.
- `IQueryBuilder.Execute() ([]arrow.Record, error)` and the slice-based
  `ExecuteAsync`; Go cannot overload these by return type or context argument.
- `QueryConfig`, `VectorSearch`, `ConnectionOptions`, `StorageOptions`,
  `S3Config`, `AzureConfig`, `GCSConfig`, `LocalConfig`, `IndexInfo` and
  `AddDataOptions` field layouts and JSON tags.
- `DistanceTypeL2 = 0`, `DistanceTypeCosine = 1`, `DistanceTypeDot = 2`,
  `DistanceTypeHamming = 3`.
- `go 1.21` and Arrow Go v17 type identity.

Additive capabilities use companion interfaces with distinct receivers:
`ITableQueryV2` / `IQueryBuilderV2`, `ITableVectorQuery` /
`IVectorQueryBuilderV2`, `ITableIndexManagement`, `ITableMergeInsert`,
`ITableOptimize`, `ITableUpdateExpr`, `ITableTimeTravel`,
`ITableSchemaEvolve` and `ITablePrewarmIndex`. Existing external
implementations of the published interfaces keep compiling.

## What changed

- **Pure Go.** `rust/`, cgo and the static-library build are gone. The SDK loads
  the published `lancedb-c` shared library with `purego`, uses `CGO_ENABLED=0`
  and keeps the loaded library mapped for the process lifetime.
- **Native artifacts.** The binding pins a published `lancedb-c` preview. The
  release notes name the exact C release, bundled SDK and Lance versions. The
  shared library is delivered as one compiled-in companion module per target,
  `go.lancedb.io/lancedb-go/artifacts/<target>`; the root module requires all
  five and build-tagged selectors link exactly one. The first `Connect`
  verifies the payload, extracts it into a private per-user cache and checks
  the runtime version; no network access happens at runtime. Explicit override:
  `LANCEDB_C_LIBRARY_PATH`; cache override: `LANCEDB_C_CACHE_DIR`; pre-seed
  with:
  `go run github.com/lancedb/lancedb-go/pkg/cmd/lancedb-go-native@latest`.
  Concurrent installers serialize cache publication with a per-target OS file
  lock. Only a digest mismatch replaces an entry; a transient open/read/sharing
  error is reported, not treated as corruption.
- **Platforms.** Linux amd64/arm64, Windows amd64/arm64 and macOS Apple Silicon.
  Intel macOS (`darwin/amd64`) is removed; its published release is not
  produced any more.
- **Connection options.** The published `ConnectionOptions` and structured
  storage types are unchanged. Top-level `Region` and
  `ReadConsistencyInterval` remain ignored, as they were in v0.1.2. Structured
  S3 credentials and profile now go through the per-connection C setter instead
  of mutating process environment variables. Use
  `ConnectWithStorageOptions` for arbitrary flat options such as
  `aws_profile`, `aws_region`, `dynamodb_endpoint` and
  `aws_s3_addressing_style`.
- **Writes.** `Add` and `AddRecords` remain append operations.
  `AddDataOptions.Mode` / `WriteModeOverwrite` remain accepted but ignored.
  Multi-batch input is one native reader and one write.
- **Updates.** `Update` accepts JSON scalar values (string, number, bool, nil),
  rejects composite values, quotes strings with single quotes and forwards the
  caller's SQL predicate. `ITableUpdateExpr.UpdateExpr` forwards raw expressions
  and returns affected rows plus the commit version.
- **Queries.** The published slice API preserves batch boundaries. The additive
  context/single-record API concatenates all result batches and returns one
  record. Schemas, input records and results cross the boundary as Arrow IPC
  files, not JSON.
- **Indexes.** `IndexParams.DistanceType` is an optional
  `*DistanceType`; omission stays distinct from L2. `IndexStatistics.Loss` is
  removed because the Rust SDK removed training loss and `IndexStatistics` was
  never in v0.1.2. SDK index type strings are mapped back to the published Go
  spellings (`IvfPq`, `BTree`, and so on).
- **Capabilities.** The additive interfaces cover FTS/hybrid search, rerankers
  and normalization, row IDs, fast search, postfilter, vector tuning, extended
  merge insert, optimize actions and stats, time travel and tags, schema
  evolution, index statistics/drop/wait/prewarm, and parameterized index
  creation.
- **Wait behavior.** `WaitForIndex` with an empty names slice takes one index
  snapshot and waits for those names; the SDK still rejects a wait budget above
  its documented two-hour limit. Positive sub-millisecond budgets floor to one
  millisecond; zero means no upper bound.

## Not changed

- No stable release is created by this migration.
- The old `rust/` cgo FFI gets no new capabilities.
- No new capability is invented in Go: unsupported backend behavior is returned
  as the SDK's error.

## Legacy entry points keep v0.1.2 behavior

The migration adds execution behind the published interfaces where v0.1.2 had a
placeholder, but it keeps the old call contracts where behavior was observable:

- `Update` still requires a non-empty predicate. Values are encoded as JSON
  scalars first, so named strings, `time.Time`, typed nil pointers and custom
  `json.Marshaler` results keep their old meaning; strings become SQL
  single-quoted literals. Invalid `json.Number`, non-finite floats and composite
  values fail before any write. Predicates, column names, update values,
  `UpdateExpr` expressions and merge-insert keys/conditions reject embedded NUL
  bytes before native execution, because a C string would otherwise truncate
  them. `UpdateExpr` remains the raw-SQL path and allows an empty filter for a
  whole-table update.
- `Select` keeps an explicit zero limit as zero rows. Negative limits and
  offsets are ignored, a negative vector `K` falls back to the ordinary query
  path, and a vector query ignores `Offset`. A `VectorSearch` with a nil
  vector is ignored, matching v0.1.2's JSON `null`; an explicitly empty vector
  still fails. Non-finite vector values fail during the legacy JSON encoding
  before branch selection, regardless of `K` or `Limit`. A query with both
  `VectorSearch` and `FTSSearch` remains a vector query when the vector branch
  is selected. Legacy FTS still
  returns the v0.1.2 `Full-text search is not currently supported` error; use
  `SelectV2` or the additive query interfaces for FTS.
- Legacy map rows keep the old JSON conversion: `Int8`/`Int16`, `UInt8`,
  `Binary`, timestamp and variable-length `List` values use the old
  `Unsupported type: ...` placeholders, fixed-size lists are decoded, and
  non-finite floats become nil. `GetAllIndexes` keeps the `FTS` spelling.
- `Add(ctx, nil, nil)` on a closed table returns an error. Closing the
  connection makes the legacy async query put an error on its error channel.
  Legacy `Connect` and `Select` continue to execute with an already-canceled
  context; additive `SelectV2` honors cancellation.
- The legacy `Connect` structured S3 path only reads access key, secret,
  session token, region and profile, as v0.1.2 did. Endpoint, path-style,
  anonymous and TLS switches are explicit through `ConnectWithStorageOptions`.
- `QueryV2`/`SelectV2` are additive. `Postfilter` and `Rerank` belong to the
  vector builder; a direct `QueryConfigV2` with either but no vector query is
  rejected. An explicitly empty `VectorQuery` is rejected instead of becoming
  a table scan. `RerankerConfig.MarshalJSON` keeps the main wire shape for
  callers that serialize it themselves.
- `RuntimeVersion` is the only public native diagnostic retained; cache paths,
  source observation and installation live in the internal package, and the
  preinstall command is `pkg/cmd/lancedb-go-native`.
