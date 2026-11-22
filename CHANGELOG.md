# Changelog

## Unreleased

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
- Adopt the current SDK SQL dialect. Read [MIGRATION.md](MIGRATION.md): update
  and delete predicates can succeed while affecting different rows.

### Removed

- Remove `rust/`, cgo, the static-library build and the native release workflow.
- Remove Intel macOS (`darwin/amd64`) support.
- Remove the unpublished `IndexStatistics.Loss` field.
