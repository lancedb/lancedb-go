#!/usr/bin/env bash
# Cold offline native consumer acceptance for the checkout under test.
#
# This is the pre-release counterpart of run.sh: it builds the consumer from
# the repository checkout (so the exact branch head is exercised), then proves
# that the compiled-in published companion payload loads with an empty private
# cache, no LANCEDB_C_LIBRARY_PATH and the network disabled. A second run uses
# the explicit override and must extract nothing.
#
# The tag-based run.sh lane remains the published-module acceptance after a
# root release tag exists.
set -euo pipefail

ROOT_MODULE=github.com/lancedb/lancedb-go
ARTIFACT_PREFIX=go.lancedb.io/lancedb-go/artifacts
TARGET_GOOS=${TARGET_GOOS:?TARGET_GOOS is required}
TARGET_GOARCH=${TARGET_GOARCH:?TARGET_GOARCH is required}
LIBRARY_NAME=${LIBRARY_NAME:?LIBRARY_NAME is required}
EXPECTED_UNAME=${EXPECTED_UNAME:-}
REPO_DIR=${REPO_DIR:-${GITHUB_WORKSPACE:-$(pwd)}}
REPO_DIR=$(cd "$REPO_DIR" && pwd -P)
EVIDENCE_DIR=${EVIDENCE_DIR:-${RUNNER_TEMP:-/tmp}/offline-checkout-evidence}
SLUG="${TARGET_GOOS}-${TARGET_GOARCH}"

mkdir -p "$EVIDENCE_DIR"
EVIDENCE_DIR=$(cd "$EVIDENCE_DIR" && pwd -P)
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
VERIFY="$SCRIPT_DIR/verify.py"
PYTHON=${PYTHON:-}
if [[ -z $PYTHON ]]; then
  PYTHON=$(command -v python3 || command -v python || true)
fi
[[ -n $PYTHON ]] || { echo "ERROR: python3 or python is required" >&2; exit 1; }
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/offline-checkout.XXXXXX")
SUMMARY="$EVIDENCE_DIR/summary.txt"
COMMANDS="$EVIDENCE_DIR/commands.log"
: > "$SUMMARY"
: > "$COMMANDS"

GO_ENV=(
  -u GOPRIVATE -u GONOPROXY -u GONOSUMDB -u GONOSUMCHECK -u GOINSECURE -u GOFLAGS
  GOENV=off GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0
)

finish() {
  local rc=$?
  cleanup_network_off
  printf '%s\n' "$rc" > "$EVIDENCE_DIR/exit-code.txt"
  if [[ $rc -ne 0 ]]; then
    printf 'result=FAIL\nexit_code=%s\n' "$rc" >> "$SUMMARY"
    printf 'offline checkout consumer FAILED (exit %s); work kept at %s\n' "$rc" "$WORK" | tee -a "$SUMMARY" >&2
  fi
}
trap finish EXIT

log() {
  printf '%s\n' "$*" | tee -a "$SUMMARY"
}

record() {
  printf '%s=%s\n' "$1" "$2" | tee -a "$SUMMARY"
}

fail() {
  printf 'ERROR: %s\n' "$*" | tee -a "$SUMMARY" >&2
  exit 1
}

run_go() {
  printf '+ cwd=%s go %s\n' "$REPO_DIR" "$*" >> "$COMMANDS"
  (cd "$REPO_DIR" && env "${GO_ENV[@]}" go "$@")
}

# ---------------------------------------------------------------------------
# Preconditions

actual_goos=$(run_go env GOOS)
actual_goarch=$(run_go env GOARCH)
[[ $actual_goos == "$TARGET_GOOS" && $actual_goarch == "$TARGET_GOARCH" ]] \
  || fail "host target is $actual_goos/$actual_goarch, want $TARGET_GOOS/$TARGET_GOARCH"
actual_uname=$(uname -m)
if [[ -n $EXPECTED_UNAME && $actual_uname != "$EXPECTED_UNAME" ]]; then
  fail "uname -m is $actual_uname, want $EXPECTED_UNAME"
fi
record host "$(uname -s) $actual_uname"
if [[ $(uname -s) == Linux ]]; then
  glibc=$(getconf GNU_LIBC_VERSION 2>/dev/null | awk '{print $2}' || true)
  if [[ -n $glibc ]] && ! awk -v v="$glibc" 'BEGIN {
      split(v, have, "."); split("2.35", want, ".");
      exit !(have[1] + 0 > want[1] + 0 ||
             (have[1] + 0 == want[1] + 0 && have[2] + 0 >= want[2] + 0))
    }'; then
    fail "the published Linux libraries require glibc 2.35 or newer; this runner has $glibc"
  fi
  record glibc "${glibc:-unknown}"
fi

# ---------------------------------------------------------------------------
# Build the checkout consumer and verify its artifact dependency.

BIN="$WORK/consumer"
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) BIN="$WORK/consumer.exe" ;;
esac
run_go build -o "$BIN" ./scripts/offline-consumer/consumer
[[ -f $BIN ]] || fail "checkout consumer was not built: $BIN"

ARTIFACT_VERSION=$(run_go list -m -f '{{.Version}}' "$ARTIFACT_PREFIX/$SLUG")
[[ -n $ARTIFACT_VERSION && $ARTIFACT_VERSION != "<nil>" ]] || fail "could not resolve $ARTIFACT_PREFIX/$SLUG"
ARTIFACT_DIR=$(run_go list -m -f '{{.Dir}}' "$ARTIFACT_PREFIX/$SLUG")
MODCACHE=$(run_go env GOMODCACHE)
if command -v cygpath >/dev/null 2>&1; then
  MODCACHE=$(cygpath -u "$MODCACHE")
fi
ARTIFACT_ZIP="$MODCACHE/cache/download/$ARTIFACT_PREFIX/$SLUG/@v/$ARTIFACT_VERSION.zip"
[[ -f $ARTIFACT_ZIP ]] || fail "artifact module archive is missing: $ARTIFACT_ZIP"
record artifact_version "$ARTIFACT_VERSION"

MANIFEST_SOURCE="$REPO_DIR/pkg/internal/native/manifest.go"
[[ -f $MANIFEST_SOURCE ]] || fail "manifest is missing: $MANIFEST_SOURCE"
EXPECTED_RUNTIME=$(sed -n 's/.*ReleaseVersion = "\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
RELEASE_TAG=$(sed -n 's/.*ReleaseTag = "v\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
RELEASE_COMMIT=$(sed -n 's/.*ReleaseCommit = "\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
[[ -n $EXPECTED_RUNTIME ]] || fail "manifest does not declare ReleaseVersion"
[[ -n $RELEASE_TAG ]] || fail "manifest does not declare ReleaseTag"
[[ -n $RELEASE_COMMIT ]] || fail "manifest does not declare ReleaseCommit"
record runtime_version "$EXPECTED_RUNTIME"
record release_tag "v$RELEASE_TAG"
record release_commit "$RELEASE_COMMIT"

ARTIFACT_JSON="$EVIDENCE_DIR/artifact.json"
printf '+ %s verify.py artifact\n' "$PYTHON" >> "$COMMANDS"
"$PYTHON" "$VERIFY" artifact \
  --zip "$ARTIFACT_ZIP" \
  --mod "$ARTIFACT_DIR/go.mod" \
  --module "$ARTIFACT_PREFIX/$SLUG" \
  --expected-target "$TARGET_GOOS/$TARGET_GOARCH" \
  --expected-library "$LIBRARY_NAME" \
  --expected-upstream-commit "$RELEASE_COMMIT" \
  --out "$ARTIFACT_JSON" 2>&1 | tee -a "$SUMMARY"
ARTIFACT_SIZE=$("$PYTHON" -c 'import json,sys; print(json.load(open(sys.argv[1]))["uncompressed_size"])' "$ARTIFACT_JSON")
ARTIFACT_SHA=$("$PYTHON" -c 'import json,sys; print(json.load(open(sys.argv[1]))["uncompressed_sha256"])' "$ARTIFACT_JSON")
record artifact_uncompressed_size "$ARTIFACT_SIZE"
record artifact_uncompressed_sha256 "$ARTIFACT_SHA"

env "${GO_ENV[@]}" go version -m "$BIN" > "$EVIDENCE_DIR/binary-modules.txt"
"$PYTHON" "$VERIFY" binary \
  --metadata "$EVIDENCE_DIR/binary-modules.txt" \
  --root-module "$ROOT_MODULE" \
  --root-kind mod \
  --root-version "(devel)" \
  --artifact-prefix "$ARTIFACT_PREFIX" \
  --slug "$SLUG" \
  --artifact-version "$ARTIFACT_VERSION" \
  --goos "$TARGET_GOOS" \
  --goarch "$TARGET_GOARCH" 2>&1 | tee -a "$SUMMARY"

# ---------------------------------------------------------------------------
# Cold default Connect with no network, an empty cache and no override.

if ! detect_network_off "$BIN"; then
  fail "no network-off mechanism is available on $(uname -s); refusing to claim an offline run"
fi
record network_disable "$NET_KIND"
COLD_CACHE="$WORK/cold-cache"
COLD_OUT="$EVIDENCE_DIR/consumer-cold.out"
if run_consumer "$BIN" "$COLD_CACHE" "$WORK/cold-db" "$COLD_OUT" cold-default; then
  :
else
  fail "cold default consumer failed; output in $COLD_OUT"
fi
expect_field "$COLD_OUT" runtime_version "$EXPECTED_RUNTIME"
expect_field "$COLD_OUT" rows_created 3
expect_field "$COLD_OUT" rows_selected 3
expect_field "$COLD_OUT" failure_path table_not_found
expect_field "$COLD_OUT" rows_after_failure 3
expect_field "$COLD_OUT" closed true
"$PYTHON" "$VERIFY" cache \
  --cache-root "$COLD_CACHE" \
  --release-version "$EXPECTED_RUNTIME" \
  --slug "$SLUG" \
  --library "$LIBRARY_NAME" \
  --size "$ARTIFACT_SIZE" \
  --sha "$ARTIFACT_SHA" 2>&1 | tee -a "$SUMMARY"
COLD_LIBRARY="$COLD_CACHE/lancedb-go/native/$EXPECTED_RUNTIME/$SLUG/$LIBRARY_NAME"
record cold_cache_library "$COLD_LIBRARY"

# ---------------------------------------------------------------------------
# Explicit override still works and extracts nothing into a fresh cache.

OVERRIDE_CACHE="$WORK/override-cache"
OVERRIDE_OUT="$EVIDENCE_DIR/consumer-override.out"
mkdir -p "$OVERRIDE_CACHE"
printf '+ run=override LANCEDB_C_LIBRARY_PATH=%s %s -uri %s\n' \
  "$COLD_LIBRARY" "${NET_OFF[*]}" "$BIN" "$WORK/override-db" >> "$COMMANDS"
set +e
env \
  LANCEDB_C_CACHE_DIR="$OVERRIDE_CACHE" \
  LANCEDB_C_LIBRARY_PATH="$COLD_LIBRARY" \
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 NO_PROXY= \
  "${NET_OFF[@]}" "$BIN" -uri "$WORK/override-db" > "$OVERRIDE_OUT" 2>&1
override_rc=$?
set -e
cat "$OVERRIDE_OUT" | tee -a "$SUMMARY"
[[ $override_rc == 0 ]] || fail "explicit override consumer failed; output in $OVERRIDE_OUT"
expect_field "$OVERRIDE_OUT" runtime_version "$EXPECTED_RUNTIME"
expect_field "$OVERRIDE_OUT" failure_path table_not_found
if [[ -n $(find "$OVERRIDE_CACHE" -type f ! -name '.lock' -print -quit 2>/dev/null) ]]; then
  fail "explicit override extracted a library into the fresh cache"
fi
record override_cache_files "$(find "$OVERRIDE_CACHE" -type f 2>/dev/null | wc -l | tr -d ' ')"

log "result=PASS"
record result PASS
