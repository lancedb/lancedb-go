#!/usr/bin/env bash
# Publish a generated Go module origin tree to an R2/S3 bucket.
#
# Credentials come only from the ambient AWS CLI environment/config. This
# script never reads, prints, or asks for them. Immutable version objects use
# create-only conditional PutObject. The @v/list object is merged with the
# existing object and updated with If-Match ETag, so an older version is never
# lost by a later publish.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: publish-go-module-origin.sh --manifest PATH --bucket NAME --endpoint URL
       [--version VERSION] [--key-prefix PREFIX] [--dry-run]

Immutable objects are `.info`, `.mod`, `.zip`, and discovery HTML. The
`@v/list` object is merged with existing versions and updated only with an
ETag condition. Every manifest object is validated locally before any network
write. There is no overwrite fallback: a conflicting immutable object aborts.
EOF
}

manifest=""
bucket=""
endpoint=""
key_prefix=""
version=""
dry_run=0
while [[ $# -gt 0 ]]; do
  case "$1" in
  --manifest) manifest=$2; shift 2 ;;
  --bucket) bucket=$2; shift 2 ;;
  --endpoint) endpoint=$2; shift 2 ;;
  --version) version=$2; shift 2 ;;
  --key-prefix) key_prefix=$2; shift 2 ;;
  --dry-run) dry_run=1; shift ;;
  -h | --help) usage; exit 0 ;;
  *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n $manifest && -n $bucket && -n $endpoint ]] || { usage >&2; exit 2; }
[[ -f $manifest ]] || { echo "missing manifest: $manifest" >&2; exit 1; }
staging=$(cd "$(dirname "$manifest")" && pwd)
command -v aws >/dev/null || { echo "aws CLI is required" >&2; exit 1; }
command -v sha256sum >/dev/null || { echo "sha256sum is required" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

object_key() {
  local key=$1
  if [[ -n $key_prefix ]]; then
    printf '%s/%s' "${key_prefix%/}" "$key"
  else
    printf '%s' "$key"
  fi
}

stage_path() {
  local key=$1
  if [[ -n $key_prefix ]]; then
    key=${key#"${key_prefix%/}/"}
  fi
  printf '%s/%s' "$staging" "$key"
}

aws_cli() {
  if [[ ${AWS_NO_SIGN_REQUEST:-0} == 1 ]]; then
    aws --no-sign-request "$@"
  else
    aws "$@"
  fi
}

head_key() {
  aws_cli s3api head-object --bucket "$bucket" --endpoint-url "$endpoint" --key "$1" >/dev/null 2>&1
}

get_object() {
  local key=$1 body=$2 meta=$3
  # Remove previous bodies first: a failed GET must never leave a readable
  # stale file behind for a later hash.
  rm -f "$body" "$meta"
  aws_cli s3api get-object --bucket "$bucket" --endpoint-url "$endpoint" \
    --key "$key" "$body" > "$meta"
}

object_etag() {
  python3 - "$1" <<'PY'
import json
import sys
# R2 requires the ETag condition value to retain its double quotes.
print(json.load(open(sys.argv[1], encoding="utf-8")).get("ETag", ""))
PY
}

object_sha256() {
  local key=$1 body=$2 meta=$3
  # Explicit status propagation: command substitution does not inherit
  # errexit, so a failed GET must return non-zero rather than hash a previous
  # temp file. Callers must check this status.
  if ! get_object "$key" "$body" "$meta"; then
    return 1
  fi
  if [[ ! -f $body ]]; then
    return 1
  fi
  sha256sum "$body" | awk '{print $1}'
}

put_object() {
  local key=$1 path=$2 ctype=$3 cache=$4 hash=$5
  shift 5
  local args=(s3api put-object --bucket "$bucket" --endpoint-url "$endpoint"
    --key "$key" --body "$path" --content-type "$ctype" --cache-control "$cache")
  if [[ -n $hash ]]; then
    args+=(--metadata "sha256=$hash")
  fi
  aws_cli "${args[@]}" "$@" >/dev/null
}

publish_immutable() {
  local key=$1 path=$2 ctype=$3 cache=$4 hash=$5
  if head_key "$key"; then
    local existing
    if ! existing=$(object_sha256 "$key" "$tmp/existing" "$tmp/existing.json"); then
      echo "read existing object failed: $key" >&2
      exit 1
    fi
    [[ $existing == "$hash" ]] || {
      echo "immutable object conflict: $key (existing $existing, expected $hash)" >&2
      exit 1
    }
    printf 'unchanged %s\n' "$key"
    return
  fi
  if ! put_object "$key" "$path" "$ctype" "$cache" "$hash" --if-none-match '*'; then
    if ! head_key "$key"; then
      echo "conditional create failed and object is absent: $key" >&2
      exit 1
    fi
    local existing
    if ! existing=$(object_sha256 "$key" "$tmp/created" "$tmp/created.json"); then
      echo "read concurrently created object failed: $key" >&2
      exit 1
    fi
    [[ $existing == "$hash" ]] || {
      echo "conditional create conflict: $key" >&2
      exit 1
    }
    printf 'created concurrently %s\n' "$key"
    return
  fi
  local uploaded
  if ! uploaded=$(object_sha256 "$key" "$tmp/uploaded" "$tmp/uploaded.json"); then
    echo "read back uploaded object failed: $key" >&2
    exit 1
  fi
  [[ $uploaded == "$hash" ]] || { echo "uploaded hash mismatch: $key" >&2; exit 1; }
  printf 'published %s\n' "$key"
}

merge_list() {
  local existing=$1 local_list=$2 merged=$3
  python3 - "$existing" "$local_list" "$merged" <<'PY'
import sys
from pathlib import Path

versions = set()
for path in sys.argv[1:3]:
    p = Path(path)
    if p.exists():
        for line in p.read_text().splitlines():
            line = line.strip()
            if line:
                versions.add(line)
Path(sys.argv[3]).write_text("\n".join(sorted(versions)) + ("\n" if versions else ""))
PY
}

publish_list() {
  local key=$1 path=$2 ctype=$3 cache=$4
  local attempt body="$tmp/list.body" meta="$tmp/list.json" merged="$tmp/list.merged" etag
  for attempt in 1 2 3; do
    if head_key "$key"; then
      if ! get_object "$key" "$body" "$meta"; then
        echo "read list object failed: $key" >&2
        exit 1
      fi
      etag=$(object_etag "$meta")
      [[ -n $etag ]] || { echo "list object has no ETag: $key" >&2; exit 1; }
      merge_list "$body" "$path" "$merged"
      if cmp -s "$body" "$merged"; then
        printf 'unchanged %s\n' "$key"
        return
      fi
      if put_object "$key" "$merged" "$ctype" "$cache" "" --if-match "$etag"; then
        :
      else
        printf 'list update conflict, retrying %s (attempt %d)\n' "$key" "$attempt" >&2
        continue
      fi
    else
      merge_list /dev/null "$path" "$merged"
      if put_object "$key" "$merged" "$ctype" "$cache" "" --if-none-match '*'; then
        :
      else
        printf 'list create conflict, retrying %s (attempt %d)\n' "$key" "$attempt" >&2
        continue
      fi
    fi
    if ! get_object "$key" "$body" "$meta"; then
      echo "read back list object failed: $key" >&2
      exit 1
    fi
    if cmp -s "$body" "$merged"; then
      printf 'published %s\n' "$key"
      return
    fi
    printf 'list verify mismatch, retrying %s (attempt %d)\n' "$key" "$attempt" >&2
  done
  echo "list update failed after retries: $key" >&2
  exit 1
}

# Validate every manifest entry and local byte before any AWS request.
python3 - "$manifest" "$staging" "$version" > "$tmp/objects.tsv" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
staging = pathlib.Path(sys.argv[2]).resolve()
expected_version = sys.argv[3]
data = json.loads(manifest_path.read_text(encoding="utf-8"))
assert data.get("module_prefix") == "go.lancedb.io/lancedb-go/artifacts", data.get("module_prefix")
assert data.get("version", "").startswith("v"), data.get("version")
if expected_version:
    assert data["version"] == expected_version, (data["version"], expected_version)
objects = data.get("objects")
assert isinstance(objects, list) and objects, "manifest has no objects"
keys = set()
rows = []
for obj in objects:
    key = obj.get("key", "")
    assert key and not key.startswith("/") and ".." not in key.split("/") and "\\" not in key, key
    assert key not in keys, f"duplicate key {key}"
    keys.add(key)
    assert obj.get("content_type"), key
    assert obj.get("cache_control"), key
    assert isinstance(obj.get("immutable"), bool), key
    path = (staging / key).resolve()
    assert str(path).startswith(str(staging) + "/"), key
    assert path.is_file(), f"missing {path}"
    raw = path.read_bytes()
    digest = hashlib.sha256(raw).hexdigest()
    assert len(raw) == obj.get("size"), (key, len(raw), obj.get("size"))
    assert digest == obj.get("sha256"), (key, digest, obj.get("sha256"))
    if key.startswith("lancedb-go/artifacts/"):
        assert obj["immutable"] is True, key
    elif "/@v/list" in key:
        assert obj["immutable"] is False, key
    elif key.endswith(".info") or key.endswith(".mod") or key.endswith(".zip"):
        assert obj["immutable"] is True, key
    else:
        raise AssertionError(f"unexpected key {key}")
    priority = 0
    if "/@v/list" in key:
        priority = 1
    elif key.startswith("lancedb-go/artifacts/"):
        priority = 2
    rows.append((priority, key, obj["sha256"], obj["size"], obj["kind"], "true" if obj["immutable"] else "false", obj["content_type"], obj["cache_control"]))
rows.sort()
print("priority\tkey\tsha256\tsize\tkind\timmutable\tcontent_type\tcache_control")
for row in rows:
    print("\t".join(str(item) for item in row))
PY

while IFS=$'\t' read -r priority key hash size kind immutable ctype cache; do
  [[ $key == "key" ]] && continue
  path=$(stage_path "$key")
  published_key=$(object_key "$key")
  if [[ $dry_run == 1 ]]; then
    printf 'would publish %s %s (%s bytes, immutable=%s)\n' "$published_key" "$hash" "$size" "$immutable"
    continue
  fi
  if [[ $immutable == true ]]; then
    publish_immutable "$published_key" "$path" "$ctype" "$cache" "$hash"
  else
    publish_list "$published_key" "$path" "$ctype" "$cache"
  fi
done < "$tmp/objects.tsv"

[[ $dry_run == 1 ]] && echo "preflight verified"
echo "publication verified"
