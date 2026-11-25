# Offline consumer acceptance

`run.sh` exercises the published SDK as an ordinary external consumer. It
resolves one exact release revision from the public Go module proxy, builds a
standalone consumer outside the checkout, and proves that the native payload is
delivered through the Go module graph instead of a runtime download.

It does not rebuild native code, use Rust, protoc, a private module proxy or a
`replace`/`go.work` file. The hosted workflow is
[`.github/workflows/offline-consumers.yml`](../../.github/workflows/offline-consumers.yml);
it runs for a root `v*` release tag or a manual dispatch that names that tag,
and never for a commit SHA or pseudo-version.

## What is checked

For the requested target, with Go 1.21 and `CGO_ENABLED=0`:

- The ordinary `go get github.com/lancedb/lancedb-go@<tag>` resolves the root
  module and all five companion modules, and the root `go.mod` requires each
  `go.lancedb.io/lancedb-go/artifacts/<target>` at one version.
- The selected artifact module archive matches its own `artifact.go` metadata:
  target, library name, compressed size and SHA-256, uncompressed size and
  SHA-256, source repository, source release, source commit and header SHA-256.
  Its `go.mod` declares `go 1.21.0` and no requirements.
- A fresh target build downloads exactly the selected artifact payload archive,
  and the built binary embeds exactly one artifact dependency with
  `CGO_ENABLED=0`.
- A cold default `Connect`, with no network access, an empty private native
  cache and no `LANCEDB_C_LIBRARY_PATH`, creates a table, reads it back,
  exercises a missing-table failure without corrupting the connection and
  closes.
- The extracted cache library is byte-for-byte the artifact payload and carries
  safe permissions.
- The explicit `LANCEDB_C_LIBRARY_PATH` override still works and extracts
  nothing into a fresh cache.

## Network isolation

Each run refuses to claim an offline check without a real isolation mechanism:

- Linux: `unshare -rn` (or passwordless `sudo -n unshare -n`) network namespace.
- macOS: `sandbox-exec` with a deny-network profile.
- Windows: no shell-level network sandbox exists on the hosted runners, so the
  run disables Go module fetch and routes `HTTP_PROXY`/`HTTPS_PROXY` through a
  dead address. The summary records `network_disable=env-blocked` for that case.

The published Linux libraries require glibc 2.35 or newer; the lane refuses to
run on older Linux hosts instead of reporting a load failure as a product bug.

## Local invocation

Run on the matching native host from the repository checkout:

```sh
ROOT_VERSION=vX.Y.Z-preview.N \
TARGET_GOOS=linux TARGET_GOARCH=amd64 \
EXPECTED_UNAME=x86_64 LIBRARY_NAME=liblancedb.so \
EVIDENCE_DIR=/tmp/offline-consumer-evidence \
bash scripts/offline-consumer/run.sh
```

For pre-publication integration only, `PUBLIC_GOPROXY` and `PUBLIC_GOSUMDB` can
point at a local fixture proxy, with `FIXTURE_MODE=1` to relax the release-tag
assertions. The production defaults are `proxy.golang.org` and
`sum.golang.org`; a real run must not set the fixture variables.

`verify.py` performs the independent archive, binary and cache checks.
`verify_test.py` exercises those checks with synthetic fixtures:

```sh
python3 scripts/offline-consumer/verify_test.py
```
