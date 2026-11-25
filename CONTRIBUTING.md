# Contributing

## Requirements

- Go 1.21 or newer.
- `CGO_ENABLED=0`.
- No Rust toolchain and no C compiler are required. The SDK loads the
  published `lancedb-c` shared library at runtime.

## Build and test

```bash
make build            # go build ./... with CGO_ENABLED=0
make test-native      # loader, cache and ABI layout tests (no native library)
make test             # full suite; uses the compiled-in native payload
make vet fmt-check
```

The first test or `Connect` extracts the compiled-in native payload into
`LANCEDB_C_CACHE_DIR` (default: the user cache directory) and verifies it
against the companion module's published digests. For an offline environment,
run `make install-native` on a machine where `go get` has already resolved the
modules and copy the cache, or point `LANCEDB_C_LIBRARY_PATH` at a library you
installed yourself.

## Authenticated S3 and AWS profile tests

The integration suite is opt-in:

```bash
# Any authenticated S3-compatible fixture. The test creates its own bucket.
export MINIO_ENDPOINT=http://127.0.0.1:19000
export MINIO_ACCESS_KEY=TESTPRIMARY
export MINIO_SECRET_KEY=primary-secret
export MINIO_BUCKET=lancedb-go-integration

# Used by the profile/region test, which starts two isolated gateways.
export VERSITYGW_BIN=/path/to/versitygw

make test-integration
```

Use synthetic credentials and loopback endpoints only. Never point these tests
at a production account or real data.

## Layout

| Path | Responsibility |
| --- | --- |
| `pkg/contracts` | Published v0.1.2 API plus additive companion interfaces and types. |
| `pkg/lancedb` | Public constructors (`Connect`, schemas, native install/version helpers). |
| `pkg/internal` | Go adapters: handles, locking, Arrow IPC conversion, lifecycle. |
| `pkg/internal/native` | Library discovery, companion-module payload extraction, SHA-256 verification, cache, purego bindings, C structs and ABI layout tests. |
| `include/lancedb.h` | Exact public C header from the pinned `lancedb-c` release. |
| `pkg/cmd/lancedb-go-native` | Extract and verify the compiled-in native payload into the cache. |

`pkg/contracts` is a compatibility boundary. Do not add a method to a published
interface or a field to a published struct to expose a new capability; add a
companion interface or a distinct options type instead. The additive query API
is separate because Go cannot overload `Execute` by argument or return type.

## Native artifact changes

`pkg/internal/native/manifest.go` pins the `lancedb-c` release tag and runtime
version, and the root `go.mod` requires the five per-target companion modules
at their published artifact version. When updating the pin, verify that the
published modules resolve through the public proxy and checksum database, that
each artifact's `SourceRelease` matches the release tag, and that the built
binary for every supported target links exactly its own artifact module. Do not
add payloads or companion module tags to this repository.

## Pull requests

Keep changes append-only. Run `make fmt-check vet test-native test` before
opening a PR, and run the integration suite when the change touches storage,
profiles, IPC, ownership or lifecycle. Include the exact commands and results
for behavior that CI cannot cover locally.
