# Changelog

## v0.2.0-preview.3

### Changed

- Update the pinned `lancedb-c` to `v0.1.0-preview.5` (LanceDB SDK
  `v0.42.0-beta.30`, Lance `v15.0.0-beta.35`, source commit
  `3106de158b8da5dcb47e2a4acc820ed1b74b3e01`).
- Distribute the native library through the Go module graph. The payload for
  each supported target is delivered by a companion module at
  `go.lancedb.io/lancedb-go/artifacts/<target>`, built from the exact
  `lancedb-c` release; the root module requires all five at
  `v0.1.0-preview.5` and build-tagged selectors link exactly the one matching
  the build. Native payloads and companion tags stay out of this repository.
- Replace the first-`Connect` HTTP download with verification and extraction of
  the compiled-in payload into the existing per-target locked cache. Cache
  repair, permissions, concurrency and the explicit `LANCEDB_C_LIBRARY_PATH`
  override are unchanged; the loader no longer uses the network at runtime.
- Point installation docs at `@latest` and the releases page, and add the
  hosted offline consumer lane that proves a cold default `Connect` on all five
  targets with no network access.

The published library SHA-256 values are `4145c061702a652019a11eb5767c292d9b01c394eb16a3850c2cec53338767fd`
(linux-amd64), `7bd7378990d540c8d4c273e48f10ec155e959a49bca9e62534c38a401bbae50b`
(linux-arm64), `727f43a57f33f0fb1955d5b2d5388817dbf720a2ace47c131b02def91994ea2d`
(darwin-arm64), `4b9f90ba0ca48775c9b90cca200e6a6c22c9e8771628da3897ee473e0e805de2`
(windows-amd64) and `99b8e6ddf4203f0b2b2d435853f134bd2d5802dcf15d5b79f126301cd84f41fe`
(windows-arm64). The archive and module identity is recorded in
`scripts/go-module-origin/v0.1.0-preview.5.identity.json`.

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
