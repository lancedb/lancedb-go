# Static Go module origin

This directory generates and publishes the per-target lancedb-go companion
modules from exact public `lancedb-c` release archives. The root module stays
`github.com/lancedb/lancedb-go`; companion modules are ordinary Go modules
served from a static origin, not Git tags and not committed payloads.

## Public layout

```text
lancedb-go/artifacts/<target>                                  # go-import HTML
modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/list
modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/<version>.info
modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/<version>.mod
modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/<version>.zip
```

The discovery object contains the three-field go-import metadata
`<module> mod https://go.lancedb.io/modules`. The query string is ignored by
the static object; no Worker is required. Targets are `linux-amd64`,
`linux-arm64`, `darwin-arm64`, `windows-amd64` and `windows-arm64`.

## Companion module contract

Each module is data-only. It contains exactly `go.mod`, `artifact.go`,
`LICENSE` and `<library>.zst`:

- Package name `artifact`; import path
  `go.lancedb.io/lancedb-go/artifacts/<target>`.
- `artifact.Payload []byte` is the zstd-compressed native library for the
  target. Importing the module embeds data; it does not link, load or execute
  native code.
- Metadata constants: `Target`, `LibraryName`, `CompressedSize`,
  `CompressedSHA256`, `UncompressedSize`, `UncompressedSHA256`, `UpstreamRepo`,
  `UpstreamCommit`, `HeaderSHA256`.
- `go 1.21.0`, matching the root module; no requirements.
- The consumer selects the module with per-target build-constrained files
  (`//go:build linux && amd64` and so on). The companion module itself carries
  no build tags, so the selection lives in the loader, not in the module.

Native payloads and companion tags never enter Git. The module version is
immutable: a native byte change, including a new `lancedb-c` release, requires a
new artifact version; published bytes are never replaced.

## Versioning

One artifact version is shared by all five modules and identifies the exact
`lancedb-c` release the payloads were packaged from: artifact version
`v0.1.0-preview.5` packages `lancedb-c v0.1.0-preview.5`. A Go-only root release
reuses the existing immutable artifact versions; a new `lancedb-c` release
produces a new artifact version with the same number.

## Identity file

`<version>.identity.json` pins the release and records the reviewed candidate
bytes:

- `version`, `time`, `module_prefix`, `base_url`, `go_version`;
- `source_repo`, `source_release`, `source_commit`, `source_tag_object`,
  `source_tag_type`, `download_base_url`;
- `header_sha256`, `license_sha256`;
- `manifest_sha256` and, per target, the archive name/checksum, the exact
  library path/checksum, the candidate `artifact.go`/`go.mod`/`LICENSE`/payload
  checksums, the module zip and go.mod h1 values, and `candidate_run_id` when
  recorded from a hosted candidate workflow run.

`fetch-release.py` refuses an archive whose published `.sha256` or internal
`SHA256SUMS` disagrees with the identity; it also requires the tag to peel to
`source_commit`, the archive manifest to name the same commit/target/version,
and the header and license hashes to match. Nothing is published from an
unrecorded candidate.

## Local generation

```sh
export PATH="$HOME/.local/go1.21.13/bin:$PATH"   # Go 1.21 for the module floor
IDENTITY=scripts/go-module-origin/v0.1.0-preview.5.identity.json \
WORK_DIR=/tmp/lancedb-go-origin \
  scripts/go-module-origin/generate-candidates.sh
```

The command downloads only the five exact archives (bounded, retried), verifies
them, builds the candidate modules, assembles the origin tree and records the
candidate identity. `WORK_DIR`, `CANDIDATES_DIR`, `DOWNLOADS_DIR`, `ORIGIN_DIR`
and `IDENTITY_OUT` control the output locations; nothing is written inside the
checkout. The output is staging data; do not commit payloads or ZIPs.

Verify locally:

```sh
ORIGIN_DIR=/tmp/lancedb-go-origin/origin \
IDENTITY=/tmp/lancedb-go-origin/candidate-identity.json \
  scripts/test-go-module-origin.sh
```

The check runs the real Go module protocol over a file proxy and a local HTTP
discovery server, compares the decompressed payloads with the recorded library
hashes and requires `go 1.21.0` in every generated `go.mod`.

## Candidate workflow

`.github/workflows/generate-go-module-origin-candidates.yml` is
`workflow_dispatch` only and needs no credentials. It regenerates the
candidates from the public archives and uploads the candidate modules, the
origin staging tree and the recorded identity as review evidence.

First run for a new release (before the identity file has candidate hashes):
dispatch with `verify=false`, copy the uploaded `candidate-identity.json` into
`scripts/go-module-origin/<version>.identity.json`, review and merge it, then
rerun with `verify=true`. The publish job repeats the same regeneration and
requires the manifest SHA-256 to equal the committed identity, so the recorded
candidate bytes are validated before any publication.

## Publisher

```sh
scripts/publish-go-module-origin.sh \
  --manifest "$ORIGIN_DIR/manifest.json" \
  --bucket "$R2_BUCKET" \
  --endpoint "$R2_S3_ENDPOINT" \
  --version "$VERSION"
```

The publisher validates every manifest key, path, size and SHA-256 before any
network request. Immutable `.info`, `.mod`, `.zip` and discovery objects use
create-only `If-None-Match: *`; a conflict aborts without overwrite. `@v/list`
is merged with existing versions and updated with `If-Match` using the exact
quoted ETag returned by R2, with bounded retries. There is no unquoted ETag or
overwrite fallback. `--dry-run` performs the full preflight without writes.

## Publish workflow and first publication

`.github/workflows/publish-go-module-origin.yml` is manual `workflow_dispatch`
only, and live writes are limited to `refs/heads/main`: the publish job
re-fetches `origin/main` and requires the checked-out commit to equal it and to
track the reviewed identity file before any credential is used. The staging job
downloads the exact public release archives, regenerates the origin tree with
no credentials, verifies the candidate identity and manifest SHA-256, checks
the protocol tree locally, and preflights the publisher. The protected
`publish` job uses the `go-module-origin` GitHub Environment (secrets
`R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`; vars `R2_BUCKET`,
`R2_S3_ENDPOINT`) and maps those credentials only on the publish and lock-probe
steps; the staging job, PR CI and the public-verification step never see them.
All privileged-job actions are pinned to the audited lance-go refs.

First publication sequence:

1. Confirm the five indefinite version-prefix Bucket Locks are active
   (`modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/v` for all five
   targets). The central thread owns this confirmation.
2. Merge the reviewed identity and workflow; dispatch
   `generate-go-module-origin-candidates.yml` on `main` with `verify=true` and
   confirm the recorded identity and manifest hash.
3. Dispatch `publish-go-module-origin.yml` on `main` with `publish=true`.
   `verify_protections` defaults to true, so the first publication always runs
   the lock probe without any extra input.
4. The workflow publishes `.info`/`.mod`/`.zip` first, then merges `@v/list`,
   then writes discovery; the lock probe and the public proxy/sumdb verification
   follow. A failed probe fails the publication run.

Subsequent dispatches may set `verify_protections=false` once the locks have
been probed for that publication line.

### Lock probe

`scripts/verify-origin-locks.sh` uses the smallest retained `.info` object of
the exact version being published. It reads the object back and byte-compares
it with the staged bytes, then:

1. PUTs the identical reviewed bytes with no create-only precondition and
   requires HTTP 409 with the `ObjectLockedByBucketPolicy` code, retaining the
   actual denial response in the publication log;
2. DELETEs the object and requires the same exact denial;
3. reads the object back a final time and byte-compares it with the staged
   bytes.

A transport failure, an access-denied response (403), any other status and any
other error code are not accepted as proof. After an unexpected overwrite or
delete result the probe performs an authoritative readback: a missing object
(HTTP 404 with `NoSuchKey`) is immediately re-created from the exact staged
bytes with `If-None-Match: *` and the readback is verified, and the run fails
reporting the actual outcome. An unreadable response is never treated as "not
found" and readback hashes only a fully received current object, so a failed
GET cannot reuse a previous temp file. Credentials reach curl through a
mode-0600 netrc file, never through argv, and are never printed.

### Public verification

`scripts/verify-origin-public.sh` runs after publication without credentials.
It runs `go mod download` from a dedicated temporary probe module with fresh
`GOMODCACHE`/`GOCACHE` per attempt, `GOENV=off`, `GOWORK=off` and the
repository's `go.mod`/`go.sum` out of scope. For every module it checks the
discovery HTML, resolves the exact version through
`https://proxy.golang.org` with `GOSUMDB=sum.golang.org` enabled, requires
`Version`, `Sum` and `GoModSum` to equal the manifest exactly, byte-compares
the payload inside the public zip with the staged payload, and then resolves
the same version through `GOPROXY=direct` to exercise the discovery path and
sumdb. Each target has its own bounded deadline with retries, and the script
refuses to verify the public origin with the checksum database disabled.

## PR checks

`.github/workflows/go-module-origin-checks.yml` runs the generator unit tests,
shell/Python syntax checks, an end-to-end fixture build (a synthetic release
repository, archive set, candidate modules and origin tree), the local
protocol/discovery verification and consumer cross-compiles, and the publisher
and lock-probe fixtures against a local S3 mock: exact denial success, plus
negative cases for transport failure, a non-lock 403, an accepted overwrite,
an accepted delete, a delete that removes the object but loses the response, a
failed GET with identical bytes, and a missing object. It also exercises the
public-verification retry and refusal paths. It never accesses R2.
