# Changelog

## v0.2.0-preview.3

### Changed

- Distribute the native library through the Go module graph. The payload for
  each supported target is delivered by a companion module at
  `go.lancedb.io/lancedb-go/artifacts/<target>`, built from the exact
  `lancedb-c` release; the root module requires all five and build-tagged
  selectors link exactly the one matching the build. Native payloads and
  companion tags stay out of this repository.
- Replace the first-`Connect` HTTP download with verification and extraction of
  the compiled-in payload into the existing per-target locked cache. Cache
  repair, permissions, concurrency and the explicit `LANCEDB_C_LIBRARY_PATH`
  override are unchanged; the loader no longer uses the network at runtime.
- Point installation docs at `@latest` and the releases page, and add the
  hosted offline consumer lane that proves a cold default `Connect` on all five
  targets with no network access.

### Removed

- Remove `LANCEDB_C_RELEASE_BASE_URL`, the release archive download, and the
  archive/mirror handling they supported.

## v0.2.0-preview.2

### Changed

- Update the pinned `lancedb-c` to `v0.1.0-preview.3` (LanceDB SDK
  `v0.42.0-beta.26`, Lance `v15.0.0-beta.28`) with the verified archive and
  library digests.
- Point preview installation at the releases page and exercise the external
  `examples` module in the no-cgo CI matrix.

## v0.2.0-preview.1

### Changed

- Migrate the SDK to pure Go over the published `lancedb-c` shared library
  (`CGO_ENABLED=0`, no Rust or C toolchain), with verified native-artifact
  discovery, an explicit override and a version check.
- Preserve the published v0.1.2 API, Arrow Go v17 type identity, struct layouts
  and distance values. Additive capabilities use companion interfaces rather
  than expanding the published interfaces.
- Add Arrow IPC schema/record/result transport, multi-batch writes, merge
  insert, FTS/hybrid queries, rerankers, optimize, time travel, schema
  evolution, rich index management and raw-expression updates.
- Apply storage options per connection, including `aws_profile`, `aws_region`
  and an independent `dynamodb_endpoint`, instead of mutating process
  environment variables.
- Adopt the SDK SQL dialect. Read [MIGRATION.md](MIGRATION.md): update
  and delete predicates can succeed while affecting different rows.

### Removed

- Remove `rust/`, cgo, the static-library build and the native release workflow.
- Remove Intel macOS (`darwin/amd64`) support.
- Remove the unpublished `IndexStatistics.Loss` field.
