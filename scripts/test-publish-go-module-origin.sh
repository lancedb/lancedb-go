#!/usr/bin/env bash
# Exercise the publisher and the first-publication lock probe against a local
# S3 mock: repeat, immutable conflict, @v/list merge, the exact 409
# ObjectLockedByBucketPolicy denial, and every negative outcome (transport
# failure, non-lock error, accepted overwrite, accepted delete, delete that
# removes the object but loses the response, failed GET, missing object) must
# fail the probe without leaving the object damaged.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
origin=${ORIGIN_DIR:?ORIGIN_DIR must point at a generated origin tree}
manifest="$origin/manifest.json"
version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$manifest")
module_prefix=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["module_prefix"])' "$manifest")

work=$(mktemp -d)
server_pid=""
cleanup() {
  [[ -n ${server_pid:-} ]] && kill "$server_pid" 2>/dev/null || true
  chmod -R u+w "$work" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

export AWS_NO_SIGN_REQUEST=1
export AWS_DEFAULT_REGION=auto
export AWS_EC2_METADATA_DISABLED=true

start_mock() {
  local root=$1 port=$2 log=$3
  shift 3
  python3 "$repo_root/scripts/go-module-origin/mock-s3.py" --root "$root" --port "$port" --log "$log" "$@" > "$work/server-$port.log" 2>&1 &
  server_pid=$!
  for _ in $(seq 1 100); do
    if curl -s -o /dev/null "http://127.0.0.1:$port/"; then
      return 0
    fi
    sleep 0.2
  done
  cat "$work/server-$port.log" >&2
  return 1
}

stop_mock() {
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  server_pid=""
}

publish=( "$repo_root/scripts/publish-go-module-origin.sh" )

# --- Publisher behavior without locks. ---
store="$work/store"
log="$work/requests.log"
start_mock "$store" 18790 "$log"

"${publish[@]}" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/first.log" 2>&1
grep -F 'publication verified' "$work/first.log" >/dev/null
first_puts=$(grep -c '^PUT ' "$log")
[[ $first_puts -eq 25 ]] || { echo "first publish PUT count $first_puts, want 25" >&2; exit 1; }

"${publish[@]}" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/repeat.log" 2>&1
grep -F 'publication verified' "$work/repeat.log" >/dev/null
second_puts=$(grep -c '^PUT ' "$log")
[[ $second_puts -eq $first_puts ]] || { echo "repeat publish added PUTs: $second_puts != $first_puts" >&2; exit 1; }

# Immutable conflict must abort and must not overwrite the conflicting bytes.
target_zip=$(python3 -c '
import json,sys
manifest = json.load(open(sys.argv[1]))
zips = [obj for obj in manifest["objects"] if obj["key"].endswith(".zip")]
print(zips[0]["key"])' "$manifest")
zip_path="$store/$target_zip"
printf 'conflict bytes\n' > "$zip_path"
before_put=$(grep -c '^PUT ' "$log")
set +e
"${publish[@]}" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/conflict.log" 2>&1
conflict_rc=$?
set -e
[[ $conflict_rc -ne 0 ]] || { echo "publisher accepted an immutable conflict" >&2; exit 1; }
grep -F 'immutable object conflict' "$work/conflict.log" >/dev/null
[[ $(cat "$zip_path") == 'conflict bytes' ]] || { echo "publisher overwrote conflicting bytes" >&2; exit 1; }
after_put=$(grep -c '^PUT ' "$log")
[[ $after_put -eq $before_put ]] || { echo "conflict publish wrote objects" >&2; exit 1; }
cp "$origin/$target_zip" "$zip_path"
stop_mock

# --- Publisher readback: a failed GET for a later identical .info object must
# not hash the previous object's temp file and report a false success. ---
python3 - "$manifest" "$origin/stale-manifest.json" <<'PY'
import json
import pathlib
import sys

source = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
infos = sorted((obj for obj in source["objects"] if obj["key"].endswith(".info")), key=lambda obj: obj["key"])
assert len(infos) >= 2, infos
first, second = infos[0], infos[1]
if pathlib.Path(sys.argv[1]).parent.joinpath(first["key"]).read_bytes() != pathlib.Path(sys.argv[1]).parent.joinpath(second["key"]).read_bytes():
    raise SystemExit("fixture .info objects are not byte-identical")
manifest = {
    "version": source["version"],
    "module_prefix": source["module_prefix"],
    "base_url": source["base_url"],
    "objects": [first, second],
}
pathlib.Path(sys.argv[2]).write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
print("stale-read fixture:", first["key"], "then", second["key"])
PY
first_info=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["objects"][0]["key"])' "$origin/stale-manifest.json")
second_info=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["objects"][1]["key"])' "$origin/stale-manifest.json")
start_mock "$store" 18790 "$work/requests-stale.log" --fail-get "$second_info"
before_stale_put=$(grep -c '^PUT ' "$work/requests-stale.log" 2>/dev/null || true)
set +e
"${publish[@]}" --manifest "$origin/stale-manifest.json" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/stale-get.log" 2>&1
stale_rc=$?
set -e
stop_mock
[[ $stale_rc -ne 0 ]] || { echo "stale-read: publisher passed when a later identical .info GET failed" >&2; cat "$work/stale-get.log" >&2; exit 1; }
grep -F "read existing object failed: $second_info" "$work/stale-get.log" >/dev/null || { cat "$work/stale-get.log" >&2; exit 1; }
after_stale_put=$(grep -c '^PUT ' "$work/requests-stale.log" 2>/dev/null || true)
[[ $after_stale_put -eq $before_stale_put ]] || { echo "stale-read: publisher wrote objects after the failed readback" >&2; exit 1; }
cmp "$store/$second_info" "$origin/$second_info" || { echo "stale-read: object bytes changed" >&2; exit 1; }
echo "publisher stale-read fixture: ok"

# --- @v/list merge under normal and reversed publication order. ---
list_key="modules/${module_prefix}/linux-amd64/@v/list"
list_prefix="$work/list-prefix"
mkdir -p "$list_prefix/$(dirname "$list_key")"
list_file="$list_prefix/$list_key"
write_list_manifest() {
  local list_version=$1 size hash
  printf '%s\n' "$list_version" > "$list_file"
  size=$(wc -c < "$list_file" | tr -d ' ')
  hash=$(sha256sum "$list_file" | awk '{print $1}')
  cat > "$list_prefix/manifest.json" <<EOF
{
  "version": "$list_version",
  "module_prefix": "$module_prefix",
  "base_url": "https://go.lancedb.io",
  "go_version": "1.21.0",
  "objects": [
    {
      "key": "$list_key",
      "kind": "list",
      "module": "$module_prefix/linux-amd64",
      "version": "$list_version",
      "target": "linux-amd64",
      "sha256": "$hash",
      "size": $size,
      "content_type": "text/plain; charset=utf-8",
      "cache_control": "public, max-age=60",
      "immutable": false
    }
  ]
}
EOF
}

store2="$work/store-list"
log2="$work/requests-list.log"
start_mock "$store2" 18790 "$log2"
list_publish=( "${publish[@]}" --manifest "$list_prefix/manifest.json" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 )

write_list_manifest "$version"
"${list_publish[@]}" > "$work/list-create.log" 2>&1
grep -F 'publication verified' "$work/list-create.log" >/dev/null
grep -E "PUT .*@v/list 200 .*if-none-match=\*" "$log2" >/dev/null || { echo "list create did not use create-only" >&2; exit 1; }
mapfile -t versions < <(cat "$store2/$list_key")
[[ ${versions[*]} == "$version" ]] || { printf 'list create: %s\n' "${versions[*]}" >&2; exit 1; }

test_version="v9.0.0-test"
write_list_manifest "$test_version"
"${list_publish[@]}" > "$work/list-forward.log" 2>&1
grep -F 'publication verified' "$work/list-forward.log" >/dev/null
grep -E "PUT .*@v/list 200 .*if-match=\"" "$log2" >/dev/null || { echo "list update did not send a quoted If-Match ETag" >&2; exit 1; }
mapfile -t versions < <(cat "$store2/$list_key")
[[ ${versions[*]} == "$version $test_version" ]] || { printf 'forward list merge: %s\n' "${versions[*]}" >&2; exit 1; }

write_list_manifest "$version"
"${list_publish[@]}" > "$work/list-reverse.log" 2>&1
grep -F 'publication verified' "$work/list-reverse.log" >/dev/null
mapfile -t versions < <(cat "$store2/$list_key")
[[ ${versions[*]} == "$version $test_version" ]] || { printf 'reverse list merge lost a version: %s\n' "${versions[*]}" >&2; exit 1; }
stop_mock

# --- Lock probe fixtures. ---
lock_prefixes=()
for target in linux-amd64 linux-arm64 darwin-arm64 windows-amd64 windows-arm64; do
  lock_prefixes+=(--lock-prefix "modules/${module_prefix}/${target}/@v/v")
done
probe_key=$(python3 -c '
import json,re,sys
manifest = json.load(open(sys.argv[1]))
pattern = re.compile(r"^modules/.*/@v/" + re.escape(sys.argv[2]) + r"\.info$")
keys = sorted(obj["key"] for obj in manifest["objects"] if pattern.fullmatch(obj["key"]))
assert keys, "no .info object for the version"
print(keys[0])' "$manifest" "$version")
staged_hash=$(sha256sum "$origin/$probe_key" | awk '{print $1}')
probe=( "$repo_root/scripts/verify-origin-locks.sh" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" )
publish_args=( --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" )

run_expect_probe_failure() {
  # run_expect_probe_failure LABEL MOCK_ARGS...
  local label=$1
  shift
  local store="$work/store-$label" log="$work/log-$label.log"
  start_mock "$store" 18790 "$log" "${lock_prefixes[@]}" "$@"
  "${publish[@]}" "${publish_args[@]}" > "$work/publish-$label.log" 2>&1
  grep -F 'publication verified' "$work/publish-$label.log" >/dev/null || { cat "$work/publish-$label.log" >&2; exit 1; }
  set +e
  "${probe[@]}" > "$work/probe-$label.log" 2>&1
  local rc=$?
  set -e
  stop_mock
  [[ $rc -ne 0 ]] || { echo "$label: probe passed when it must fail" >&2; cat "$work/probe-$label.log" >&2; exit 1; }
  if grep -F 'lock probe: PASS' "$work/probe-$label.log" >/dev/null; then
    echo "$label: probe reported PASS" >&2
    cat "$work/probe-$label.log" >&2
    exit 1
  fi
  local actual
  actual=$(sha256sum "$store/$probe_key" 2>/dev/null | awk '{print $1}')
  [[ $actual == "$staged_hash" ]] || { echo "$label: probe left the object bytes changed (${actual:-missing})" >&2; exit 1; }
}

# Positive: the exact R2 denial for both attempts passes the probe.
store3="$work/store-locks"
log3="$work/requests-locks.log"
start_mock "$store3" 18790 "$log3" "${lock_prefixes[@]}"
"${publish[@]}" "${publish_args[@]}" > "$work/lock-publish.log" 2>&1
grep -F 'publication verified' "$work/lock-publish.log" >/dev/null
"${probe[@]}" > "$work/lock-probe.log" 2>&1
grep -F 'lock probe: PASS' "$work/lock-probe.log" >/dev/null || { cat "$work/lock-probe.log" >&2; exit 1; }
grep -F 'HTTP 409 ObjectLockedByBucketPolicy' "$work/lock-probe.log" >/dev/null || { echo "probe log does not retain the exact lock-denial response" >&2; cat "$work/lock-probe.log" >&2; exit 1; }
grep -c 'denied with HTTP 409 ObjectLockedByBucketPolicy' "$work/lock-probe.log" | grep -qx 2 || { echo "probe did not record both denials" >&2; cat "$work/lock-probe.log" >&2; exit 1; }
stop_mock

# Negative: transport failure on both attempts plus a successful GET must fail.
run_expect_probe_failure transport-both --locked-write-mode transport --locked-delete-mode transport
grep -F 'overwrite produced no HTTP response' "$work/probe-transport-both.log" >/dev/null || { cat "$work/probe-transport-both.log" >&2; exit 1; }

# Negative: a non-lock 403 denial must not count as success.
run_expect_probe_failure forbidden --locked-write-status 403 --locked-write-code AccessDenied --locked-delete-status 403 --locked-delete-code AccessDenied
grep -F 'HTTP 403 code AccessDenied' "$work/probe-forbidden.log" >/dev/null || { cat "$work/probe-forbidden.log" >&2; exit 1; }

# Negative: an accepted overwrite of identical bytes must fail the probe.
run_expect_probe_failure overwrite-accepted --locked-write-mode accept
grep -F 'overwrite response was HTTP 200' "$work/probe-overwrite-accepted.log" >/dev/null || { cat "$work/probe-overwrite-accepted.log" >&2; exit 1; }

# Negative: an accepted delete triggers immediate repair and failure.
run_expect_probe_failure delete-accepted --locked-delete-mode accept
grep -F 'FAIL delete was accepted' "$work/probe-delete-accepted.log" >/dev/null || { cat "$work/probe-delete-accepted.log" >&2; exit 1; }
grep -F 'object restored and readback verified' "$work/probe-delete-accepted.log" >/dev/null || { cat "$work/probe-delete-accepted.log" >&2; exit 1; }

# Negative: a delete that removes the object but loses the response must be
# detected by the authoritative readback, repaired and fail.
run_expect_probe_failure delete-transport-remove --locked-delete-mode transport-remove
grep -F 'delete produced no HTTP response' "$work/probe-delete-transport-remove.log" >/dev/null || { cat "$work/probe-delete-transport-remove.log" >&2; exit 1; }
grep -F 'object is missing; restoring the exact staged bytes' "$work/probe-delete-transport-remove.log" >/dev/null || { cat "$work/probe-delete-transport-remove.log" >&2; exit 1; }
grep -F 'object restored and readback verified' "$work/probe-delete-transport-remove.log" >/dev/null || { cat "$work/probe-delete-transport-remove.log" >&2; exit 1; }

# Negative: a benign failed GET while the object bytes are identical must fail
# explicitly instead of hashing a previous temp file and reporting PASS. The
# object is published with a healthy read path first, then the read fails.
store_failed_get="$work/store-failed-get"
start_mock "$store_failed_get" 18790 "$work/log-failed-get-publish.log" "${lock_prefixes[@]}"
"${publish[@]}" "${publish_args[@]}" > "$work/publish-failed-get.log" 2>&1
grep -F 'publication verified' "$work/publish-failed-get.log" >/dev/null
stop_mock
start_mock "$store_failed_get" 18790 "$work/log-failed-get.log" "${lock_prefixes[@]}" --fail-get "$probe_key"
set +e
"${probe[@]}" > "$work/probe-failed-get.log" 2>&1
failed_get_rc=$?
set -e
stop_mock
[[ $failed_get_rc -ne 0 ]] || { echo "failed-get: probe passed" >&2; cat "$work/probe-failed-get.log" >&2; exit 1; }
grep -F 'lock probe: PASS' "$work/probe-failed-get.log" >/dev/null && { echo "failed-get: probe reported PASS" >&2; cat "$work/probe-failed-get.log" >&2; exit 1; }
grep -F 'unreadable no HTTP response' "$work/probe-failed-get.log" >/dev/null || { cat "$work/probe-failed-get.log" >&2; exit 1; }
grep -F 'could not be determined' "$work/probe-failed-get.log" >/dev/null || { cat "$work/probe-failed-get.log" >&2; exit 1; }
failed_get_hash=$(sha256sum "$store_failed_get/$probe_key" | awk '{print $1}')
[[ $failed_get_hash == "$staged_hash" ]] || { echo "failed-get: object bytes changed" >&2; exit 1; }

# Negative: a missing object fails the pre-probe readback (and is restored).
store7="$work/store-missing"
start_mock "$store7" 18790 "$work/log-missing.log" "${lock_prefixes[@]}"
set +e
"${probe[@]}" > "$work/probe-missing.log" 2>&1
missing_rc=$?
set -e
stop_mock
[[ $missing_rc -ne 0 ]] || { echo "missing object: probe passed" >&2; exit 1; }
grep -F 'pre-probe readback did not show the reviewed object' "$work/probe-missing.log" >/dev/null || { cat "$work/probe-missing.log" >&2; exit 1; }


# --- Public verification logic against the local origin server. ---
start_mock_server_on_18888() {
  python3 "$repo_root/scripts/go-module-origin/test-server.py" --root "$origin" --port 18888 > "$work/public-server-$1.log" 2>&1 &
  server_pid=$!
  for _ in $(seq 1 50); do
    if curl -fsS -o /dev/null "http://127.0.0.1:18888/lancedb-go/artifacts/linux-amd64?go-get=1" 2>/dev/null; then
      return 0
    fi
    sleep 0.2
  done
  return 1
}

start_mock_server_on_18888 first >/dev/null || { echo "cannot start origin server" >&2; exit 1; }
ORIGIN_BASE_URL=http://127.0.0.1:18888 \
PROXY_URL=http://127.0.0.1:18888/modules \
SUMDB=off \
DIRECT_PROBE=0 \
  "$repo_root/scripts/verify-origin-public.sh" \
    --manifest "$manifest" --origin "$origin" --timeout-seconds 60 --interval-seconds 2 \
    > "$work/public-verify.log" 2>&1
grep -F 'public verification: PASS' "$work/public-verify.log" >/dev/null || { cat "$work/public-verify.log" >&2; exit 1; }
stop_mock

# --- Public verification refusal and bounded failure. ---
set +e
SUMDB=off "$repo_root/scripts/verify-origin-public.sh" \
  --manifest "$manifest" --origin "$origin" > "$work/public-refuse.log" 2>&1
refuse_rc=$?
set -e
[[ $refuse_rc -ne 0 ]] || { echo "public verification accepted sumdb=off for the public origin" >&2; exit 1; }
grep -F 'refusing to verify the public origin with the checksum database disabled' "$work/public-refuse.log" >/dev/null || { cat "$work/public-refuse.log" >&2; exit 1; }

start_mock_server_on_18888 second >/dev/null || { echo "cannot start origin server" >&2; exit 1; }
set +e
ORIGIN_BASE_URL=http://127.0.0.1:18888 \
PROXY_URL=http://127.0.0.1:18890 \
DIRECT_PROBE=0 \
  timeout 60 "$repo_root/scripts/verify-origin-public.sh" \
    --manifest "$manifest" --origin "$origin" --timeout-seconds 2 --interval-seconds 1 \
    > "$work/public-bounded.log" 2>&1
bounded_rc=$?
set -e
stop_mock
[[ $bounded_rc -ne 0 ]] || { echo "public verification passed with a dead proxy" >&2; exit 1; }
grep -F 'did not resolve within the 2s per-target deadline' "$work/public-bounded.log" >/dev/null || { cat "$work/public-bounded.log" >&2; exit 1; }

echo "publisher repeat/conflict/list-merge/lock-probe/public-verification: PASS"