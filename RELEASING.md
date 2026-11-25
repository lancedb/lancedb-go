# Releasing native companion modules

This is the maintainer sequence for publishing the per-target companion
modules used by the pure-Go runtime. The root module stays
`github.com/lancedb/lancedb-go`; the native payloads are published as ordinary
Go modules at
`go.lancedb.io/lancedb-go/artifacts/<target>@<artifact-version>`, served from
the static origin described in
[scripts/go-module-origin/README.md](scripts/go-module-origin/README.md). This
document does not itself authorize a tag or a publication.

The native libraries come from exact public `lancedb-c` release archives. The
five archives, their published checksums, the packaged manifest, the C header
and the extracted libraries are all verified against the committed identity
file before any byte is published. No payload and no companion-module tag is
ever added to Git.

## Release units and order

```text
root module          github.com/lancedb/lancedb-go   tag vX.Y.Z-preview.N
linux/amd64 module   go.lancedb.io/lancedb-go/artifacts/linux-amd64   <artifact-version>
linux/arm64 module   go.lancedb.io/lancedb-go/artifacts/linux-arm64   <artifact-version>
darwin/arm64 module  go.lancedb.io/lancedb-go/artifacts/darwin-arm64  <artifact-version>
windows/amd64 module go.lancedb.io/lancedb-go/artifacts/windows-amd64 <artifact-version>
windows/arm64 module go.lancedb.io/lancedb-go/artifacts/windows-arm64 <artifact-version>
```

Publish in this order:

1. Verify the native source release and choose the artifact version.
2. Generate and verify the candidate modules and the origin staging tree.
3. Confirm the five indefinite version-prefix Bucket Locks (central thread).
4. Publish the static modules and resolve their exact versions through the
   public proxy and checksum database.
5. Wire the loader to the published versions in the follow-up change, verify a
   real `CGO_ENABLED=0` consumer offline, and only then cut the authorized root
   tag.

Root and artifact versions are independent. The artifact version identifies the
exact `lancedb-c` release the payloads were packaged from, shared by all five
modules; a Go-only root release reuses the existing immutable artifact
versions.

## 1. Choose and verify the native input

The committed identity file is the packaging pin. For `lancedb-c`
`v0.1.0-preview.5` the record is
`scripts/go-module-origin/v0.1.0-preview.5.identity.json`; its
`source_commit` is the peeled commit, and every per-target archive checksum and
library checksum comes from that publication. Do not substitute another
archive for the pin you are verifying.

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

## 3. Publish

Confirm the central thread's lock confirmation covers all five indefinite
rules `modules/go.lancedb.io/lancedb-go/artifacts/<target>/@v/v`. Then dispatch
`.github/workflows/publish-go-module-origin.yml` on `main` with
`publish=true`. The workflow accepts live writes only from `refs/heads/main`,
re-fetches `origin/main` and requires the checked-out commit to equal it and to
track the reviewed identity file before any credential is used, and pins its
privileged-job actions to audited refs. The lock probe always runs and cannot
be disabled by an input, so the first publication probes one retained `.info`
object of the exact version: it requires the service to deny an unconditional overwrite and a
delete with HTTP 409 `ObjectLockedByBucketPolicy`, never accepts a transport
failure or a 403 as proof, repairs the exact staged bytes immediately if an
authoritative readback shows the object missing, byte-compares the readback and
fails the run if the lock is not enforcing. Publication acceptance fails with
the probe.

The workflow uses the `go-module-origin` GitHub Environment
(`R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`; vars `R2_BUCKET`,
`R2_S3_ENDPOINT`) and maps credentials only on the publish and probe steps.
Artifact module versions are immutable. Publish `.info`, `.mod` and `.zip`
first, then the merged `@v/list`, then discovery last. Never create a Git tag
for an artifact module and never move a published static version.

After publication the workflow runs `go mod download` from a dedicated
temporary probe module with fresh caches and the repository's `go.mod`/
`go.sum` out of scope, resolves all five modules through `proxy.golang.org`
with `GOSUMDB=sum.golang.org` enabled, compares the public `Sum`/`GoModSum`
with the manifest h1 values exactly, byte-compares the payload inside the
public zip with the staged bytes, and re-resolves through `GOPROXY=direct` to
exercise the discovery metadata. Each target has its own bounded deadline with
retries. A missing checksum entry or a version that only resolves from a local
cache is a stop condition.

## 4. Evidence

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

Release-specific facts (the pinned C release, bundled SDK, artifact digests)
belong in the root release notes and CHANGELOG. Evergreen docs use `@latest` or
link to the releases page and must not copy the current release's tag, native
mapping, sizes or hashes.

## 5. Root release and rollback

Only after the exact artifact version resolves publicly should the follow-up
loader change adopt it, build `CGO_ENABLED=0` consumers on all five targets
without network access, and update the user-facing documentation for the
release. The root release is then cut under the project's release ownership
rules; it is not cut while native alignment or public resolution is unresolved.

Published artifact versions are append-only. If a preview is wrong, cut a new
artifact version (or a new root preview) that pins corrected behavior; do not
move, overwrite or delete a published version.
