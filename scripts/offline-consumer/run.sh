#!/usr/bin/env bash
# Offline consumer acceptance for one published lancedb-go target.
#
# The checkout supplies the harness sources and the source oracle. All consumer
# modules, module caches, build caches, binaries and SDK caches live in a fresh
# temporary directory and come from an ordinary public `go get`.
#
# The lane proves, for the requested target:
#   * the ordinary public module graph resolves and the built binary embeds
#     exactly the matching go.lancedb.io/lancedb-go/artifacts/<target> module;
#   * the downloaded artifact payload matches its own published metadata;
#   * a cold default Connect, with no network access, an empty private native
#     cache and no LANCEDB_C_LIBRARY_PATH, creates, uses, fails and closes;
#   * the extracted cache library matches the artifact payload byte for byte;
#   * the explicit LANCEDB_C_LIBRARY_PATH override still works and extracts
#     nothing into a fresh cache.
#
# It does not rebuild native code, publish modules or accept a commit SHA.
set -euo pipefail

ROOT_MODULE=${ROOT_MODULE:-github.com/lancedb/lancedb-go}
ARTIFACT_PREFIX=${ARTIFACT_PREFIX:-go.lancedb.io/lancedb-go/artifacts}
ROOT_VERSION=${ROOT_VERSION:?ROOT_VERSION is required}
TARGET_GOOS=${TARGET_GOOS:?TARGET_GOOS is required}
TARGET_GOARCH=${TARGET_GOARCH:?TARGET_GOARCH is required}
LIBRARY_NAME=${LIBRARY_NAME:?LIBRARY_NAME is required}
EXPECTED_UNAME=${EXPECTED_UNAME:-}
REPO_DIR=${REPO_DIR:-${GITHUB_WORKSPACE:-$(pwd)}}
REPO_DIR=$(cd "$REPO_DIR" && pwd -P)
EVIDENCE_DIR=${EVIDENCE_DIR:-${RUNNER_TEMP:-/tmp}/offline-consumer-evidence}
FIXTURE_MODE=${FIXTURE_MODE:-0}
PUBLIC_GOPROXY=${PUBLIC_GOPROXY:-https://proxy.golang.org}
PUBLIC_GOSUMDB=${PUBLIC_GOSUMDB:-sum.golang.org}
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
CONSUMER_SOURCE="$REPO_DIR/scripts/offline-consumer/consumer/main.go"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/offline-consumer.XXXXXX")
SUMMARY="$EVIDENCE_DIR/summary.txt"
COMMANDS="$EVIDENCE_DIR/commands.log"
: > "$SUMMARY"
: > "$COMMANDS"

# GOPROXY and GOSUMDB are overridable only so the lane can run against a local
# fixture proxy before the modules are public. The production workflow uses the
# defaults, which require the public proxy and checksum database.
GO_ENV=(
  -u GOPRIVATE -u GONOPROXY -u GONOSUMDB -u GONOSUMCHECK -u GOINSECURE -u GOFLAGS
  GOENV=off GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0
  GOPROXY="$PUBLIC_GOPROXY" GOSUMDB="$PUBLIC_GOSUMDB"
)
if [[ $FIXTURE_MODE == 1 ]]; then
  GO_ENV=( "${GO_ENV[@]}" GOSUMDB=off GOFLAGS=-mod=mod )
fi

finish() {
  local rc=$?
  printf '%s\n' "$rc" > "$EVIDENCE_DIR/exit-code.txt"
  if [[ $rc -ne 0 ]]; then
    printf 'result=FAIL\nexit_code=%s\n' "$rc" >> "$SUMMARY"
    printf 'offline-consumer acceptance FAILED (exit %s); work kept at %s\n' "$rc" "$WORK" | tee -a "$SUMMARY" >&2
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
  local stage=$1 modcache=$2 buildcache=$3
  shift 3
  printf '+ stage=%s GOMODCACHE=%s GOCACHE=%s go %s\n' \
    "$stage" "$modcache" "$buildcache" "$*" >> "$COMMANDS"
  env "${GO_ENV[@]}" GOMODCACHE="$modcache" GOCACHE="$buildcache" go "$@"
}

run_verify() {
  printf '+ %s verify.py %s\n' "$PYTHON" "$*" >> "$COMMANDS"
  "$PYTHON" "$VERIFY" "$@"
}

snapshot_module_cache() {
  local label=$1 modcache=$2 destination=$3
  : > "$destination"
  local root="$modcache/cache/download/$ROOT_MODULE" artifactRoot="$modcache/cache/download/$ARTIFACT_PREFIX"
  [[ -d $root ]] || fail "module cache has no $ROOT_MODULE download directory: $modcache"
  {
    for base in "$root" "$artifactRoot"; do
      [[ -d $base ]] || continue
      find "$base" -type f \( -name '*.zip' -o -name '*.mod' -o -name '*.info' \) -print
    done | LC_ALL=C sort
  } | while IFS= read -r path; do
    printf '%s\t%s\n' "$label" "${path#"$modcache/cache/download/"}"
  done >> "$destination"
}

artifact_zips_in() {
  local label=$1 inventory=$2
  awk -F'\t' -v l="$label" -v p="$ARTIFACT_PREFIX/" \
    '$1 == l && index($2, p) == 1 && $2 ~ /\.zip$/ { print $2 }' "$inventory" | LC_ALL=C sort -u
}

required_artifacts() {
  awk -v p="$ARTIFACT_PREFIX" '
    /^require \($/ { block = 1; next }
    block && /^\)/ { block = 0; next }
    block && $2 ~ /^v/ { print $1 "@" $2; next }
    /^require [^(]/ { print $2 "@" $3 }
  ' "$1" | awk -v p="$ARTIFACT_PREFIX" 'index($0, p "/") == 1' | LC_ALL=C sort -u
}

# ---------------------------------------------------------------------------
# Preconditions

case "$ROOT_VERSION" in
  *-fixture) [[ $FIXTURE_MODE == 1 ]] || fail "fixture revision $ROOT_VERSION requires FIXTURE_MODE=1" ;;
  v[0-9]*) ;;
  *) fail "ROOT_VERSION must be an official release tag, got: $ROOT_VERSION" ;;
esac

actual_goos=$(env "${GO_ENV[@]}" go env GOOS)
actual_goarch=$(env "${GO_ENV[@]}" go env GOARCH)
[[ $actual_goos == "$TARGET_GOOS" && $actual_goarch == "$TARGET_GOARCH" ]] \
  || fail "host target is $actual_goos/$actual_goarch, want $TARGET_GOOS/$TARGET_GOARCH"
actual_uname=$(uname -m)
if [[ -n $EXPECTED_UNAME && $actual_uname != "$EXPECTED_UNAME" ]]; then
  fail "uname -m is $actual_uname, want $EXPECTED_UNAME"
fi
record host "$(uname -s) $actual_uname"
if [[ $(uname -s) == Linux ]]; then
  # The published Linux libraries require glibc 2.35 or newer. Refuse to claim
  # runtime acceptance on a runner that cannot load them.
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
# Ordinary public go get in a fresh external module.

CONSUMER="$WORK/consumer"
mkdir -p "$CONSUMER"
cp "$CONSUMER_SOURCE" "$CONSUMER/main.go"
cd "$CONSUMER"
run_go setup "$WORK/caches/setup-mod" "$WORK/caches/setup-build" mod init example.com/offline-consumer >> "$COMMANDS" 2>&1
run_go setup "$WORK/caches/setup-mod" "$WORK/caches/setup-build" get "$ROOT_MODULE@$ROOT_VERSION" 2>&1 | tee -a "$COMMANDS"
run_go setup "$WORK/caches/setup-mod" "$WORK/caches/setup-build" mod tidy 2>&1 | tee -a "$COMMANDS"
cp go.mod "$EVIDENCE_DIR/consumer-go.mod"
cp go.sum "$EVIDENCE_DIR/consumer-go.sum"

SETUP_MODCACHE="$WORK/caches/setup-mod"
SETUP_INVENTORY="$EVIDENCE_DIR/setup-module-cache.tsv"
snapshot_module_cache setup "$SETUP_MODCACHE" "$SETUP_INVENTORY"
SETUP_ZIPS="$EVIDENCE_DIR/setup-artifact-zips.txt"
artifact_zips_in setup "$SETUP_INVENTORY" > "$SETUP_ZIPS"
[[ -s $SETUP_ZIPS ]] || fail "go get downloaded no artifact payload archives"
log "setup downloaded artifact payload archives:"
sed 's/^/  /' "$SETUP_ZIPS" | tee -a "$SUMMARY"

ROOT_GO_MOD="$SETUP_MODCACHE/cache/download/$ROOT_MODULE/@v/$ROOT_VERSION.mod"
[[ -f $ROOT_GO_MOD ]] || fail "resolved root go.mod is missing: $ROOT_GO_MOD"
cp "$ROOT_GO_MOD" "$EVIDENCE_DIR/root-go.mod"
REQUIRED="$EVIDENCE_DIR/root-required-artifacts.txt"
required_artifacts "$ROOT_GO_MOD" > "$REQUIRED"
for target in linux-amd64 linux-arm64 darwin-arm64 windows-amd64 windows-arm64; do
  grep -q "^$ARTIFACT_PREFIX/$target@" "$REQUIRED" || fail "root module does not require $ARTIFACT_PREFIX/$target"
done
line_count=$(wc -l < "$REQUIRED" | tr -d ' ')
[[ $line_count == 5 ]] || fail "root module requires $line_count artifact modules, want 5: $(tr '\n' ' ' < "$REQUIRED")"
ARTIFACT_VERSION=$(awk -F@ 'NR == 1 { print $2 }' "$REQUIRED")
[[ -n $ARTIFACT_VERSION ]] || fail "could not read the artifact module version"
while IFS= read -r required; do
  [[ $required == *"@$ARTIFACT_VERSION" ]] || fail "artifact requirements disagree on version: $required"
done < "$REQUIRED"
record artifact_version "$ARTIFACT_VERSION"

SELECTED_ZIP="cache/download/$ARTIFACT_PREFIX/$SLUG/@v/$ARTIFACT_VERSION.zip"
[[ -f "$SETUP_MODCACHE/$SELECTED_ZIP" ]] \
  || fail "go get did not download the selected artifact payload: $ARTIFACT_PREFIX/$SLUG@$ARTIFACT_VERSION"
cp "$SETUP_MODCACHE/$SELECTED_ZIP" "$WORK/selected-artifact.zip"

MANIFEST_SOURCE="$SETUP_MODCACHE/$ROOT_MODULE@$ROOT_VERSION/pkg/internal/native/manifest.go"
[[ -f $MANIFEST_SOURCE ]] || fail "resolved root source is missing: $MANIFEST_SOURCE"
EXPECTED_RUNTIME=$(sed -n 's/.*ReleaseVersion = "\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
RELEASE_TAG=$(sed -n 's/.*ReleaseTag = "v\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
RELEASE_COMMIT=$(sed -n 's/.*ReleaseCommit = "\([^"]*\)".*/\1/p' "$MANIFEST_SOURCE" | head -1)
[[ -n $EXPECTED_RUNTIME ]] || fail "root source does not declare ReleaseVersion"
[[ -n $RELEASE_TAG ]] || fail "root source does not declare ReleaseTag"
[[ -n $RELEASE_COMMIT ]] || fail "root source does not declare ReleaseCommit"
record runtime_version "$EXPECTED_RUNTIME"
record release_tag "v$RELEASE_TAG"
record release_commit "$RELEASE_COMMIT"

ARTIFACT_JSON="$EVIDENCE_DIR/artifact.json"
run_verify artifact \
  --zip "$WORK/selected-artifact.zip" \
  --mod "$SETUP_MODCACHE/cache/download/$ARTIFACT_PREFIX/$SLUG/@v/$ARTIFACT_VERSION.mod" \
  --module "$ARTIFACT_PREFIX/$SLUG" \
  --expected-target "$TARGET_GOOS/$TARGET_GOARCH" \
  --expected-library "$LIBRARY_NAME" \
  --expected-upstream-commit "$RELEASE_COMMIT" \
  --out "$ARTIFACT_JSON" 2>&1 | tee -a "$SUMMARY"
ARTIFACT_SIZE=$("$PYTHON" -c 'import json,sys; print(json.load(open(sys.argv[1]))["uncompressed_size"])' "$ARTIFACT_JSON")
ARTIFACT_SHA=$("$PYTHON" -c 'import json,sys; print(json.load(open(sys.argv[1]))["uncompressed_sha256"])' "$ARTIFACT_JSON")
record artifact_uncompressed_size "$ARTIFACT_SIZE"
record artifact_uncompressed_sha256 "$ARTIFACT_SHA"

# ---------------------------------------------------------------------------
# Build the consumer against a fresh target cache. Only the selected artifact
# payload may be downloaded.

TARGET="$WORK/target"
mkdir -p "$TARGET" "$WORK/caches/target-mod" "$WORK/caches/target-build"
cp "$CONSUMER/main.go" "$TARGET/main.go"
cp "$CONSUMER/go.mod" "$TARGET/go.mod"
cp "$CONSUMER/go.sum" "$TARGET/go.sum"
cd "$TARGET"
run_go target "$WORK/caches/target-mod" "$WORK/caches/target-build" \
  build -mod=readonly -o "$WORK/consumer-bin" . 2>&1 | tee -a "$COMMANDS"

TARGET_INVENTORY="$EVIDENCE_DIR/target-module-cache.tsv"
snapshot_module_cache target "$WORK/caches/target-mod" "$TARGET_INVENTORY"
TARGET_ZIPS=$(artifact_zips_in target "$TARGET_INVENTORY")
EXPECTED_SELECTED="$ARTIFACT_PREFIX/$SLUG/@v/$ARTIFACT_VERSION.zip"
[[ $TARGET_ZIPS == "$EXPECTED_SELECTED" ]] \
  || fail "target build downloaded artifact payloads '$TARGET_ZIPS', want exactly $EXPECTED_SELECTED"
record target_artifact_zip "$TARGET_ZIPS"
log "target build downloaded exactly the selected artifact payload"

BIN="$WORK/consumer-bin"
if [[ ! -f $BIN && -f $BIN.exe ]]; then
  BIN="$BIN.exe"
fi
[[ -f $BIN ]] || fail "consumer binary was not built"
env "${GO_ENV[@]}" go version -m "$BIN" > "$EVIDENCE_DIR/binary-modules.txt"
run_verify binary \
  --metadata "$EVIDENCE_DIR/binary-modules.txt" \
  --root-module "$ROOT_MODULE" \
  --root-version "$ROOT_VERSION" \
  --artifact-prefix "$ARTIFACT_PREFIX" \
  --slug "$SLUG" \
  --artifact-version "$ARTIFACT_VERSION" \
  --goos "$TARGET_GOOS" \
  --goarch "$TARGET_GOARCH" 2>&1 | tee -a "$SUMMARY"

# ---------------------------------------------------------------------------
# Cold default Connect with no network, an empty cache and no override.

if ! detect_network_off; then
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
run_verify cache \
  --cache-root "$COLD_CACHE" \
  --release-version "$EXPECTED_RUNTIME" \
  --slug "$SLUG" \
  --library "$LIBRARY_NAME" \
  --size "$ARTIFACT_SIZE" \
  --sha "$ARTIFACT_SHA" 2>&1 | tee -a "$SUMMARY"
record cold_cache_library "$COLD_CACHE/lancedb-go/native/$EXPECTED_RUNTIME/$SLUG/$LIBRARY_NAME"

# ---------------------------------------------------------------------------
# Explicit override still works and extracts nothing into a fresh cache.

OVERRIDE_CACHE="$WORK/override-cache"
OVERRIDE_OUT="$EVIDENCE_DIR/consumer-override.out"
mkdir -p "$OVERRIDE_CACHE"
printf '+ run=override LANCEDB_C_LIBRARY_PATH=%s %s -uri %s\n' \
  "$COLD_CACHE/lancedb-go/native/$EXPECTED_RUNTIME/$SLUG/$LIBRARY_NAME" "${NET_OFF[*]}" "$BIN" "$WORK/override-db" >> "$COMMANDS"
set +e
env \
  LANCEDB_C_CACHE_DIR="$OVERRIDE_CACHE" \
  LANCEDB_C_LIBRARY_PATH="$COLD_CACHE/lancedb-go/native/$EXPECTED_RUNTIME/$SLUG/$LIBRARY_NAME" \
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
