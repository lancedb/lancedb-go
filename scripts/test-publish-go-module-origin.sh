#!/usr/bin/env bash
# Exercise the publisher and the first-publication lock probe against a local
# S3 mock: repeat, conflict, no-overwrite, @v/list merge, lock denial and the
# repair path when a delete is unexpectedly accepted.
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

# --- Lock probe when the service denies overwrite and delete. ---
store3="$work/store-locks"
log3="$work/requests-locks.log"
lock_prefixes=()
for target in linux-amd64 linux-arm64 darwin-arm64 windows-amd64 windows-arm64; do
  lock_prefixes+=(--lock-prefix "modules/${module_prefix}/${target}/@v/v")
done
start_mock "$store3" 18790 "$log3" "${lock_prefixes[@]}"
publish_args=( --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" )
"${publish[@]}" "${publish_args[@]}" > "$work/lock-publish.log" 2>&1
grep -F 'publication verified' "$work/lock-publish.log" >/dev/null
"$repo_root/scripts/verify-origin-locks.sh" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/lock-probe.log" 2>&1
grep -F 'lock probe: PASS' "$work/lock-probe.log" >/dev/null || { cat "$work/lock-probe.log" >&2; exit 1; }
grep -F 'AccessDenied' "$work/lock-probe.log" >/dev/null || { echo "probe log does not retain the lock-denial response" >&2; exit 1; }
stop_mock

# --- Lock probe repair path when a delete is unexpectedly accepted. ---
store4="$work/store-repair"
log4="$work/requests-repair.log"
start_mock "$store4" 18790 "$log4" "${lock_prefixes[@]}" --allow-locked-delete
"${publish[@]}" "${publish_args[@]}" > "$work/repair-publish.log" 2>&1
grep -F 'publication verified' "$work/repair-publish.log" >/dev/null
set +e
"$repo_root/scripts/verify-origin-locks.sh" --manifest "$manifest" --bucket lancedb-go-fixture --endpoint http://127.0.0.1:18790 --version "$version" > "$work/repair-probe.log" 2>&1
repair_rc=$?
set -e
[[ $repair_rc -ne 0 ]] || { echo "probe accepted an unlocked delete" >&2; exit 1; }
grep -F 'FAIL delete was accepted' "$work/repair-probe.log" >/dev/null || { cat "$work/repair-probe.log" >&2; exit 1; }
probe_key=$(python3 -c '
import json,re,sys
manifest = json.load(open(sys.argv[1]))
pattern = re.compile(r"^modules/.*/@v/" + re.escape(sys.argv[2]) + r"\.info$")
keys = sorted(obj["key"] for obj in manifest["objects"] if pattern.fullmatch(obj["key"]))
print(keys[0])' "$manifest" "$version")
sha256sum "$store4/$probe_key" | awk '{print $1}' > "$work/repaired.sha256"
sha256sum "$origin/$probe_key" | awk '{print $1}' > "$work/staged.sha256"
cmp "$work/repaired.sha256" "$work/staged.sha256" || { echo "repair did not restore the staged bytes" >&2; exit 1; }
stop_mock

# --- Public verification logic against the local origin server. ---
python3 "$repo_root/scripts/go-module-origin/test-server.py" --root "$origin" --port 18888 > "$work/public-server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
  if curl -fsS -o /dev/null "http://127.0.0.1:18888/lancedb-go/artifacts/linux-amd64?go-get=1" 2>/dev/null; then
    break
  fi
  sleep 0.2
done
ORIGIN_BASE_URL=http://127.0.0.1:18888 \
PROXY_URL=http://127.0.0.1:18888/modules \
SUMDB=off \
DIRECT_PROBE=0 \
  "$repo_root/scripts/verify-origin-public.sh" \
    --manifest "$manifest" --origin "$origin" --timeout-seconds 60 --interval-seconds 2 \
    > "$work/public-verify.log" 2>&1
grep -F 'public verification: PASS' "$work/public-verify.log" >/dev/null || { cat "$work/public-verify.log" >&2; exit 1; }
stop_mock

echo "publisher repeat/conflict/list-merge/lock-probe/public-verification: PASS"
