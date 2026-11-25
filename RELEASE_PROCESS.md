# Release process

`lancedb-go` is a Go module. A release is a Git tag such as
`vX.Y.Z-preview.N`; the Go module proxy serves the tagged source. The SDK does
not build or attach native libraries: the native payloads are published
separately as five per-target companion modules at
`go.lancedb.io/lancedb-go/artifacts/<target>` and are linked through the root
module's requirements.

## Before tagging

1. Confirm `pkg/internal/native/manifest.go` names the intended `lancedb-c`
   release tag and runtime version, and that the companion modules built from
   that exact release are published and resolve through the public Go module
   proxy and checksum database at their artifact version.
2. Confirm the root `go.mod` requires all five companion modules at that
   version and that each artifact module's `SourceRelease` matches
   `ReleaseTag`.
3. Confirm CI is green on the exact commit for Linux amd64/arm64, Windows
   amd64/arm64 and macOS Apple Silicon with `CGO_ENABLED=0`, including the
   external `examples` module consumer, and that the authenticated S3 and AWS
   profile/DynamoDB loopback tests pass.
4. Confirm the offline consumer lanes are green for the published revision:
   `.github/workflows/offline-consumers.yml` resolves the tag through the
   public proxy and runs a cold default `Connect` with no network access on all
   five targets.
5. Confirm the public API baseline and [MIGRATION.md](MIGRATION.md) are current.
6. Verify the module graph declares Go 1.21 and that `go mod tidy` is clean.

Keep release-specific facts (the pinned C release, bundled SDK, artifact
version and payload digests) in the release notes and CHANGELOG. User-facing
installation docs point to `@latest` or the releases page.

## Tagging

```bash
VERSION=vX.Y.Z-preview.N  # the tag being released
git tag "$VERSION"
git push origin "refs/tags/$VERSION"
```

Post-migration preview releases are owned by the maintenance loop. Stable
releases remain with Xuanwo.

## After tagging

Verify the tag resolves through the public Go module proxy, then build and run
a clean external module against the tagged version without a local `replace`.
The pushed tag also starts the offline consumer lanes; record the resolved
version, the selected artifact module and the native runtime check with the
release evidence.

## Rollback

Tags are append-only. If a published preview is wrong, cut a new preview that
pins corrected behavior or a newer artifact version; do not move or delete the
old tag and do not overwrite a published artifact module.
