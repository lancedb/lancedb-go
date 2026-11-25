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
2. Confirm `pkg/internal/native/manifest.go` pins that `lancedb-c` release tag
   and runtime version, that the root `go.mod` requires all five companion
   modules at the published artifact version, and that each artifact module's
   `SourceRelease` matches `ReleaseTag`.
3. Confirm a real `CGO_ENABLED=0` consumer runs offline on all five targets
   with an empty native cache and no explicit library path, including the
   separate `LANCEDB_C_LIBRARY_PATH` override. The tag-based
   `.github/workflows/offline-consumers.yml` lane covers the published module
   graph after the tag exists.
4. Confirm CI is green on the exact commit for Linux amd64/arm64, Windows
   amd64/arm64 and macOS Apple Silicon with `CGO_ENABLED=0`, including the
   external `examples` module consumer, and that the authenticated S3 and AWS
   profile/DynamoDB loopback tests pass.
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
pins corrected behavior or a newer `lancedb-c` artifact; do not move or delete
the old tag. The same rule applies to the immutable companion-module versions:
do not overwrite a published artifact module.
