#!/usr/bin/env bash
# Prove the first-publication bucket-lock protections for one published
# companion-module version.
#
# The probe uses one small immutable `.info` object of the exact version being
# accepted. It first reads the object back and byte-compares it with the
# reviewed staged bytes, then attempts:
#
#   1. an unconditional PUT of the identical reviewed bytes (no
#      `If-None-Match`), which a version lock must deny; and
#   2. a DELETE of the object, which a version lock must deny.
#
# If a DELETE is unexpectedly accepted, the probe immediately re-creates the
# exact staged bytes, verifies the readback, and fails the run reporting a
# failed lock. Credentials come only from the ambient AWS CLI environment and
# are never printed.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify-origin-locks.sh --manifest PATH --bucket NAME --endpoint URL --version VERSION
       [--module-prefix PREFIX]

Runs the first-publication lock probe described in the script header. The
probe object is scoped to `modules/<module-prefix>/<target>/@v/<version>.info`;
any other version is rejected.

Exit status is non-zero when the overwrite or delete attempt is accepted, or
when the final readback does not match the staged bytes.
EOF
}

manifest=""
bucket=""
endpoint=""
version=""
module_prefix="go.lancedb.io/lancedb-go/artifacts"
while [[ $# -gt 0 ]]; do
  case "$1" in
  --manifest) manifest=$2; shift 2 ;;
  --bucket) bucket=$2; shift 2 ;;
  --endpoint) endpoint=$2; shift 2 ;;
  --version) version=$2; shift 2 ;;
  --module-prefix) module_prefix=$2; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n $manifest && -n $bucket && -n $endpoint && -n $version ]] || { usage >&2; exit 2; }
[[ -f $manifest ]] || { echo "missing manifest: $manifest" >&2; exit 1; }
command -v aws >/dev/null || { echo "aws CLI is required" >&2; exit 1; }
command -v sha256sum >/dev/null || { echo "sha256sum is required" >&2; exit 1; }
staging=$(cd "$(dirname "$manifest")" && pwd)

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

aws_cli() {
  if [[ ${AWS_NO_SIGN_REQUEST:-0} == 1 ]]; then
    aws --no-sign-request "$@"
  else
    aws "$@"
  fi
}

get_object() {
  aws_cli s3api get-object --bucket "$bucket" --endpoint-url "$endpoint" --key "$1" "$2" > "$3"
}

object_sha256() {
  local key=$1 body=$2 meta=$3
  get_object "$key" "$body" "$meta"
  sha256sum "$body" | awk '{print $1}'
}

# Select the probe without trusting the caller: the object must be one
# immutable `.info` of exactly the version being accepted.
python3 - "$manifest" "$version" "$module_prefix" > "$tmp/probe.env" <<'PY'
import json
import pathlib
import re
import shlex
import sys

manifest_path = pathlib.Path(sys.argv[1])
version = sys.argv[2]
prefix = sys.argv[3].strip("/")
if not re.fullmatch(r"v[0-9][0-9A-Za-z.\-+]*", version):
    raise SystemExit(f"verify-origin-locks: version {version!r} does not look like a Go module version")
pattern = re.compile(
    "^modules/" + re.escape(prefix) + r"/[a-z0-9\-]+/@v/" + re.escape(version) + r"\.info$"
)
data = json.loads(manifest_path.read_text(encoding="utf-8"))
if data.get("version") != version:
    raise SystemExit(
        f"verify-origin-locks: manifest version {data.get('version')!r} is not the requested {version!r}"
    )
probe = None
for obj in data.get("objects", []):
    if not pattern.fullmatch(obj.get("key", "")):
        continue
    if obj.get("immutable") is not True:
        raise SystemExit(f"verify-origin-locks: probe candidate {obj['key']!r} is not immutable")
    if probe is None or obj["key"] < probe["key"]:
        probe = obj
if probe is None:
    raise SystemExit(f"verify-origin-locks: manifest has no {version}.info object under {prefix}")
print(f"probe_key={shlex.quote(probe['key'])}")
print(f"probe_sha256={shlex.quote(probe['sha256'])}")
print(f"probe_content_type={shlex.quote(probe['content_type'])}")
print(f"probe_cache_control={shlex.quote(probe['cache_control'])}")
PY
# shellcheck disable=SC1090
source "$tmp/probe.env"
[[ -n ${probe_key:-} && -n ${probe_sha256:-} ]] || { echo "probe selection failed" >&2; exit 1; }

staged_path="$staging/$probe_key"
[[ -f $staged_path ]] || { echo "staged probe object is missing: $staged_path" >&2; exit 1; }
staged_hash=$(sha256sum "$staged_path" | awk '{print $1}')
[[ $staged_hash == "$probe_sha256" ]] || { echo "staged probe hash $staged_hash, manifest says $probe_sha256" >&2; exit 1; }
echo "lock probe: key=$probe_key version=$version staged_sha256=$staged_hash"

readback=$(object_sha256 "$probe_key" "$tmp/readback-before" "$tmp/readback-before.json")
[[ $readback == "$staged_hash" ]] || { echo "lock probe: FAIL pre-probe readback $readback, want $staged_hash" >&2; exit 1; }
echo "lock probe: pre-probe readback unchanged sha256=$readback"

# 1. Unconditional overwrite of the identical reviewed bytes. There is no
# create-only condition, so only the service-side lock can deny this.
overwrite_log="$tmp/overwrite.log"
if aws_cli s3api put-object --bucket "$bucket" --endpoint-url "$endpoint" \
  --key "$probe_key" --body "$staged_path" \
  --content-type "$probe_content_type" --cache-control "$probe_cache_control" \
  --metadata "sha256=$staged_hash" > "$overwrite_log" 2>&1; then
  echo "lock probe: FAIL overwrite without a precondition was accepted" >&2
  cat "$overwrite_log" >&2
  readback=$(object_sha256 "$probe_key" "$tmp/readback-failed" "$tmp/readback-failed.json" 2>/dev/null || true)
  echo "lock probe: post-failure readback sha256=${readback:-unreadable}; object bytes were identical to the reviewed bytes" >&2
  exit 1
fi
echo "lock probe: overwrite denied by the service:"
cat "$overwrite_log"

# 2. Delete of the object. If accepted, repair immediately from staged bytes
# and fail the run: the lock is not enforcing immutability.
delete_log="$tmp/delete.log"
if aws_cli s3api delete-object --bucket "$bucket" --endpoint-url "$endpoint" \
  --key "$probe_key" > "$delete_log" 2>&1; then
  echo "lock probe: FAIL delete was accepted; repairing from staged bytes" >&2
  repaired=0
  for attempt in 1 2 3; do
    if aws_cli s3api put-object --bucket "$bucket" --endpoint-url "$endpoint" \
      --key "$probe_key" --body "$staged_path" \
      --content-type "$probe_content_type" --cache-control "$probe_cache_control" \
      --metadata "sha256=$staged_hash" --if-none-match '*' > "$tmp/repair.log" 2>&1; then
      repaired=1
      break
    fi
    if readback=$(object_sha256 "$probe_key" "$tmp/readback-repair" "$tmp/readback-repair.json" 2>/dev/null) && [[ $readback == "$staged_hash" ]]; then
      repaired=1
      break
    fi
    sleep 2
  done
  if [[ $repaired != 1 ]]; then
    echo "lock probe: FAIL delete was accepted and the object could not be re-created; staged bytes are $staged_path" >&2
    exit 1
  fi
  readback=$(object_sha256 "$probe_key" "$tmp/readback-repaired" "$tmp/readback-repaired.json")
  [[ $readback == "$staged_hash" ]] || { echo "lock probe: FAIL repaired object readback $readback, want $staged_hash" >&2; exit 1; }
  echo "lock probe: object repaired and readback unchanged sha256=$readback" >&2
  echo "lock probe: FAIL version lock did not deny delete" >&2
  exit 1
fi
echo "lock probe: delete denied by the service:"
cat "$delete_log"

readback=$(object_sha256 "$probe_key" "$tmp/readback-final" "$tmp/readback-final.json")
[[ $readback == "$staged_hash" ]] || { echo "lock probe: FAIL final readback $readback, want $staged_hash" >&2; exit 1; }
echo "lock probe: PASS key=$probe_key version=$version sha256=$readback overwrite=denied delete=denied readback=unchanged"
