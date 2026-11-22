# Release process

`lancedb-go` is a Go module. A release is a Git tag such as `v0.2.0-preview.1`;
the Go module proxy serves the tagged source. The SDK does not build or attach
native libraries: those come from the pinned `lancedb-c` release and are
downloaded and verified by the loader.

## Before tagging

1. Confirm `pkg/internal/native/manifest.go` names the intended `lancedb-c`
   release tag, full commit, every published archive and its archive/library
   SHA-256 digests.
2. Confirm CI is green on the exact commit for Linux amd64/arm64, Windows
   amd64/arm64 and macOS Apple Silicon with `CGO_ENABLED=0`, and that the
   authenticated S3 and AWS profile/DynamoDB loopback tests pass.
3. Confirm the public API baseline and [MIGRATION.md](MIGRATION.md) are current.
4. Verify the module graph declares Go 1.21 and that `go mod tidy` is clean.

## Tagging

```bash
git tag v0.2.0-preview.1
git push origin v0.2.0-preview.1
```

The first post-migration `lancedb-go` release is a preview owned by the
maintenance loop. Stable releases remain with Xuanwo.

## Rollback

Tags are append-only. If a published preview is wrong, cut a new preview that
pins corrected behavior or a newer `lancedb-c` artifact; do not move or delete
the old tag.
