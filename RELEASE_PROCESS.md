# Release process

`lancedb-go` is a Go module. A release is a Git tag such as
`vX.Y.Z-preview.N`; the Go module proxy serves the tagged source. Native
libraries are published separately as per-target companion modules from exact
`lancedb-c` release archives; [RELEASING.md](RELEASING.md) owns that sequence
and the companion-module identity rules. The root module never carries native
payloads, and no companion-module tag is created in Git.

## Before tagging

1. Confirm the intended `lancedb-c` release has been packaged into the five
   companion modules and that the exact artifact version resolves through the
   public Go proxy and checksum database. Record the resolved versions and
   hashes with the release evidence.
2. Confirm the loader pins that exact artifact version and that a real
   `CGO_ENABLED=0` consumer runs offline on all five targets, including the
   explicit `LANCEDB_C_LIBRARY_PATH` override.
3. Confirm CI is green on the exact commit for Linux amd64/arm64, Windows
   amd64/arm64 and macOS Apple Silicon with `CGO_ENABLED=0`, including the
   external `examples` module consumer, and that the authenticated S3 and AWS
   profile/DynamoDB loopback tests pass.
4. Confirm the public API baseline and [MIGRATION.md](MIGRATION.md) are current.
5. Verify the module graph declares Go 1.21 and that `go mod tidy` is clean.

Keep release-specific facts (the pinned C release, bundled SDK and artifact
digests) in the release notes and CHANGELOG. User-facing installation docs
link to `@latest` where appropriate or to the releases page.

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
Record the resolved version and the native runtime check with the release
evidence.

## Rollback

Tags are append-only. If a published preview is wrong, cut a new preview that
pins corrected behavior or a newer `lancedb-c` artifact; do not move or delete
the old tag. The same rule applies to the immutable companion-module versions.
