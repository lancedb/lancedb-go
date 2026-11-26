# Releasing lancedb-go

This is the maintainer sequence for a lancedb-go release. A release has two
independently versioned units: the root Go module
`github.com/lancedb/lancedb-go` and five immutable companion modules that carry
the native payload for the supported targets. The root tag is cut only after
the companion versions resolve publicly and the loader that selects them has
passed exact-head CI.

The native libraries come from exact public `lancedb-c` release archives. The
five archives, their published checksums, the packaged manifest, the C header
and the extracted libraries are all verified against the committed identity
file before any byte is published. No payload and no companion-module tag is
ever added to Git. This document does not itself authorize a tag or a
publication.

[scripts/go-module-origin/README.md](scripts/go-module-origin/README.md) owns
the origin layout, identity-file schema and publisher mechanics. Release
notes and [CHANGELOG.md](CHANGELOG.md) carry release-specific facts (the pinned
C release, bundled SDK, artifact version and payload digests). Evergreen docs
use `@latest` or link to the releases page and must not copy the current
release's tag, native mapping, sizes or hashes.

## Release units

```text
root module          github.com/lancedb/lancedb-go          tag vX.Y.Z-preview.N
linux/amd64 module   go.lancedb.io/lancedb-go/artifacts/linux-amd64    <artifact-version>
linux/arm64 module   go.lancedb.io/lancedb-go/artifacts/linux-arm64    <artifact-version>
darwin/arm64 module  go.lancedb.io/lancedb-go/artifacts/darwin-arm64   <artifact-version>
windows/amd64 module go.lancedb.io/lancedb-go/artifacts/windows-amd64  <artifact-version>
windows/arm64 module go.lancedb.io/lancedb-go/artifacts/windows-arm64  <artifact-version>
```

- The artifact version identifies the exact `lancedb-c` release the payloads
  were packaged from and is shared by all five modules. A Go-only root release
  reuses the existing immutable artifact versions; a new `lancedb-c` release
  produces a new artifact version.
- Artifact modules are ordinary static Go modules served from
  `https://go.lancedb.io/modules`, not Git tags. Never create a Git tag for an
  artifact module and never move a published static version. Root tags are
  append-only as well.
- The root module keeps `go 1.21.0` and the published v0.1.2 API. The
  companion modules declare `go 1.21.0` and import nothing.

## Gate order

Run the gates in this order. A failed gate stops the release; fix the cause and
restart at that gate. Do not cut the root tag while native alignment or public
resolution is unresolved.

1. Verify the native input against the committed identity (§1).
2. Generate the candidates, review them and commit the identity (§2).
3. Confirm the lock gates, publish the artifact modules and pass the lock
   probe (§3).
4. Verify the published artifact versions resolve exactly through the public
   proxy and checksum database (§4).
5. Adopt the artifact version in the loader and pass exact-head CI, including
   the offline consumer lane (§5).
6. Cut the root tag under the project's release ownership rules (§6).
7. Verify the published root module and the post-tag offline consumer lanes
   (§7).
8. Record the release evidence (§8).

## 1. Verify the native input

The committed identity file is the packaging pin. For `lancedb-c`
`v0.1.0-preview.5` the record is
`scripts/go-module-origin/v0.1.0-preview.5.identity.json`; its `source_commit`
is the peeled commit, and every per-target archive checksum and library checksum
comes from that publication. Do not substitute another archive for the pin you
are verifying.

`fetch-release.py` re-checks the tag with `git ls-remote`, downloads only the
exact assets, requires the published `.sha256` and the archive's internal
`SHA256SUMS` to match the identity, and requires the packaged `manifest.json`
to name the same commit, target, version, library, header and license. If a
tag does not peel to the pinned commit or any hash differs, stop.

## 2. Generate and verify candidates

```bash
export PATH="$HOME/.local/go1.21.13/bin:$PATH"
IDENTITY=scripts/go-module-origin/v0.1.0-preview.5.identity.json \
WORK_DIR=/tmp/lancedb-go-origin \
  scripts/go-module-origin/generate-candidates.sh
python3 scripts/go-module-origin/verify-candidate-identity.py \
  "$IDENTITY" /tmp/lancedb-go-origin/candidates
ORIGIN_DIR=/tmp/lancedb-go-origin/origin \
IDENTITY=/tmp/lancedb-go-origin/candidate-identity.json \
  scripts/test-go-module-origin.sh
```

The command downloads only the five exact archives (bounded, retried), verifies
them, builds the candidate modules, assembles the origin tree and records the
candidate identity. `WORK_DIR`, `CANDIDATES_DIR`, `DOWNLOADS_DIR`, `ORIGIN_DIR`
and `IDENTITY_OUT` control the output locations; nothing is written inside the
checkout. The output is staging data; do not commit payloads or ZIPs.

Record the candidate and manifest identity into the committed identity file
after review. A new release needs a new identity record from its own candidate
run; an identity file never describes a later version. Regenerating from the
same identity must produce the same manifest SHA-256: the publish staging job
repeats the generation and fails if the manifest does not match the committed
`manifest_sha256`.

The generated `go.mod` declares `go 1.21.0`, matching the root module. The
companion module imports nothing, contains only `go.mod`, `artifact.go`,
`LICENSE` and the compressed library, and exposes the data-only
`artifact.Payload` plus the packaging metadata constants.

The candidate workflow
`.github/workflows/generate-go-module-origin-candidates.yml` is
`workflow_dispatch` only and needs no credentials. First run for a new release
(before the identity file has candidate hashes): dispatch with `verify=false`,
copy the uploaded `candidate-identity.json` into
`scripts/go-module-origin/<version>.identity.json`, review and merge it, then
rerun with `verify=true`.

## 3. Confirm the locks and publish the artifact modules

Confirm the central thread's lock confirmation covers all five indefinite
version-prefix rules
`modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/v`. Then dispatch
`.github/workflows/publish-go-module-origin.yml` on `main` with
`publish=true`.

The workflow accepts live writes only from `refs/heads/main`, re-fetches
`origin/main` and requires the checked-out commit to equal it and to track the
reviewed identity file before any credential is used, and pins its
privileged-job actions to audited refs. The staging job downloads the exact
public release archives, regenerates the origin tree with no credentials,
verifies the candidate identity and manifest SHA-256, checks the protocol tree
locally, and preflights the publisher. The protected `publish` job uses the
`go-module-origin` GitHub Environment (`R2_ACCESS_KEY_ID`,
`R2_SECRET_ACCESS_KEY`; vars `R2_BUCKET`, `R2_S3_ENDPOINT`) and maps
credentials only on the publish and lock-probe steps; the staging job, PR CI
and the public-verification step never see them.

The lock probe always runs and cannot be disabled by an input. It uses the
smallest retained `.info` object of the exact version being published. It reads
the object back and byte-compares it with the staged bytes, then:

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
found", and readback hashes only a fully received current object.

Credentials reach curl through a mode-0600 netrc file, never through argv, and
are never printed. The object path is URI-encoded for the request (`@v` becomes
`%40v`) because curl signs the path exactly as sent, while AWS CLI and R2
canonicalize the encoded form. The signed-path fixture runs the real probe
against a local SigV4 verifier with fixed test credentials and also performs an
AWS CLI GET against the same verifier, so a raw-path regression fails with
`SignatureDoesNotMatch`.

Artifact module versions are immutable. The publisher validates every manifest
key, path, size and SHA-256 before any network request. Immutable `.info`,
`.mod`, `.zip` and discovery objects use create-only `If-None-Match: *`; a
conflict aborts without overwrite. Publish `.info`, `.mod` and `.zip` first, then
merge `@v/list` with `If-Match` using the exact quoted ETag returned by R2, then
write discovery last. There is no unquoted ETag or overwrite fallback.
`--dry-run` performs the full preflight without writes. A failed probe fails the
publication run.

## 4. Verify the published artifact modules

`.github/workflows/publish-go-module-origin.yml` runs
`scripts/verify-origin-public.sh` after publication without credentials. It
runs `go mod download` from a dedicated temporary probe module with fresh
`GOMODCACHE`/`GOCACHE` per attempt, `GOENV=off`, `GOWORK=off` and the
repository's `go.mod`/`go.sum` out of scope. For every module it checks, in this
order:

1. the discovery HTML;
2. resolution of the exact version through `https://proxy.golang.org` with
   `GOSUMDB=sum.golang.org` enabled;
3. `Version`, `Sum` and `GoModSum` equal to the manifest exactly;
4. the payload inside the public zip byte-equals the staged payload;
5. resolution of the same version through `GOPROXY=direct` to exercise the
   discovery path and sumdb.

Each target has one monotonic, fractional deadline: every discovery curl and
every proxy/direct `go mod download` gets only the remaining time, retry sleeps
are clipped to the remaining budget, and a failed target does not consume
another target's budget. curl and go run under `timeout` with a one-second
TERM-to-KILL grace for the immediate command; a timeout is reported as exit 124
or 137. This is not a general detached-descendant guarantee.

A missing checksum entry, a checksum mismatch, a payload mismatch, or a version
that only resolves from a local cache is a stop condition. If a publication is
wrong, cut a new artifact version that pins corrected bytes; do not move,
overwrite or delete a published version.

## 5. Adopt the artifact version and pass exact-head CI

Only after the exact artifact version resolves publicly may the follow-up
loader change adopt it:

- `pkg/internal/native/manifest.go` pins the `lancedb-c` release tag, peeled
  commit and runtime version;
- the root `go.mod` requires all five companion modules at the published
  artifact version;
- the build-tagged selectors link exactly the artifact module for the build
  target, and each artifact's `UpstreamCommit` equals `ReleaseCommit`.

Before opening the adoption PR, run the same external `CGO_ENABLED=0` consumer
locally with an empty native cache and no explicit library path, then repeat it
with `LANCEDB_C_LIBRARY_PATH`. The tag-based offline lane covers the published
module graph after the tag exists; it does not replace this pre-tag proof.

CI must be green on the exact head. Within each of the five target lanes,
`.github/workflows/ci.yml` runs these steps in this order:

1. `gofmt` check over every tracked Go file and `go vet ./...`;
2. loader, cache and ABI-layout tests (`go test ./pkg/internal/native`);
3. offline consumer harness self-tests (`bash -n` and `verify_test.py`);
4. cold offline default `Connect` from the checkout consumer
   (`scripts/offline-consumer/run-checkout.sh`);
5. install the native artifact (`go run ./pkg/cmd/lancedb-go-native`) and run
   the full `go test ./...` suite;
6. the explicit `LANCEDB_C_LIBRARY_PATH` override path;
7. the external `examples` module consumer (`go run ./basic_crud` and the
   cross-module tests), which proves the public import path and native runtime;
8. `go build ./...` with `CGO_ENABLED=0`.

The separate `integration` job must also pass: the authenticated S3 fixture and
the AWS profile/DynamoDB loopback tests. If the change touches the origin
scripts, publisher or workflows, the `Go module origin checks` workflow must be
green as well; it runs the generator and verifier unit tests, the end-to-end
fixture build, the local protocol/discovery check, consumer cross-compiles and
the publisher/lock-probe fixtures against a local S3 mock. It never accesses R2.

Also confirm the public API baseline and [MIGRATION.md](MIGRATION.md) are
current, the module graph declares Go 1.21, and `go mod tidy` is clean.

## 6. Cut the root tag

```bash
VERSION=vX.Y.Z-preview.N  # the tag being released
git tag "$VERSION"
git push origin "refs/tags/$VERSION"
```

Post-migration preview releases are owned by the maintenance loop. Stable
releases remain with Xuanwo. Do not cut the tag while native alignment or
public resolution is unresolved, and do not modify user-facing install
documentation before the release it describes exists.

## 7. Verify the published root module

After the tag is pushed:

1. confirm the tag resolves through the public Go module proxy;
2. build and run a clean external module against the tagged version without a
   local `replace`;
3. confirm the pushed tag started the `Offline consumers` lanes for all five
   targets and that each lane passed:
   - the ordinary public module graph resolves and the built binary embeds
     exactly the matching `go.lancedb.io/lancedb-go/artifacts/<target>` module;
   - the downloaded payload matches its published metadata;
   - a cold default `Connect`, with the network disabled, an empty private
     native cache and no `LANCEDB_C_LIBRARY_PATH`, creates, uses, fails and
     closes;
   - the extracted cache library byte-equals the artifact payload;
   - the explicit `LANCEDB_C_LIBRARY_PATH` override still works and extracts
     nothing into a fresh cache;
4. record the resolved root version, the selected artifact module and the
   native runtime check with the release evidence.

## 8. Release evidence

Record the exact objects for the release:

```text
root:             commit, tag, module path
go:               module minimum and tested Go versions
artifact:         static module paths and version, go.sum entries, proxy/sumdb identity
native:           lancedb-c repository, tag, peeled commit, header SHA-256
library:          per-target archive and library sizes and SHA-256
payload:          per-target compressed and uncompressed sizes and SHA-256
manifest:         origin manifest SHA-256 and module zip/go.mod h1 values
public archive:   proxy zip comparison and checksum-database result
public consumer:  exact root/artifact versions and offline runtime result
```

## 9. Rollback

Tags and published artifact versions are append-only. If a published preview
is wrong, cut a new preview that pins corrected behavior or a newer
`lancedb-c` artifact. Do not move or delete the old tag, and do not overwrite a
published artifact module.
