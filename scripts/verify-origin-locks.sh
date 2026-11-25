#!/usr/bin/env bash
# Prove the first-publication bucket-lock protections for one published
# companion-module version.
#
# The probe uses one small immutable `.info` object of the exact version being
# accepted. It first reads the object back and byte-compares it with the
# reviewed staged bytes, then:
#
#   1. PUTs the identical reviewed bytes with no create-only precondition and
#      requires the service to deny the overwrite with HTTP 409 and the
#      `ObjectLockedByBucketPolicy` code; and
#   2. DELETEs the object and requires the same exact denial.
#
# A transport failure, an access-denied response, any other status and any
# other error code are not accepted as proof. On an unexpected result the probe
# performs an authoritative readback; if that readback shows the object is
# missing it immediately re-creates the exact staged bytes and verifies the
# readback, and in every case the run fails reporting the actual outcome. An
# unreadable response is never treated as "not found" or as success.
#
# Credentials come only from the ambient environment. They are passed to curl
# through a mode-0600 netrc file so they never appear in argv, and they are
# never printed.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify-origin-locks.sh --manifest PATH --bucket NAME --endpoint URL --version VERSION
       [--module-prefix PREFIX]

Runs the first-publication lock probe described in the script header. The
probe object is scoped to `modules/<module-prefix>/<target>/@v/<version>.info`;
any other version is rejected.

The overwrite and delete attempts must both be denied with HTTP 409 and the
error code `ObjectLockedByBucketPolicy` (R2 Bucket Lock rules). Exit status is
non-zero for any other outcome, for an unreadable readback, or when the final
readback does not match the staged bytes.
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
command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
command -v sha256sum >/dev/null || { echo "sha256sum is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 is required" >&2; exit 1; }
staging=$(cd "$(dirname "$manifest")" && pwd)

tmp=$(mktemp -d)
netrc="$tmp/netrc"
cleanup() {
  rm -f "$netrc"
  rm -rf "$tmp"
}
trap cleanup EXIT

if [[ ${AWS_NO_SIGN_REQUEST:-0} == 1 ]]; then
  # Fixture mode: the local mock does not verify signatures. Production runs
  # must provide credentials.
  :
else
  [[ -n ${AWS_ACCESS_KEY_ID:-} && -n ${AWS_SECRET_ACCESS_KEY:-} ]] || {
    echo "verify-origin-locks: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required (or set AWS_NO_SIGN_REQUEST=1 for a fixture)" >&2
    exit 1
  }
  endpoint_host=$(python3 -c 'import sys, urllib.parse; print(urllib.parse.urlsplit(sys.argv[1]).hostname or "")' "$endpoint")
  [[ -n $endpoint_host ]] || { echo "verify-origin-locks: cannot parse endpoint host from $endpoint" >&2; exit 1; }
  umask 077
  printf 'machine %s login %s password %s\n' "$endpoint_host" "$AWS_ACCESS_KEY_ID" "$AWS_SECRET_ACCESS_KEY" > "$netrc"
  [[ $(stat -c '%a' "$netrc") == 600 ]] || { echo "verify-origin-locks: netrc must be mode 0600" >&2; exit 1; }
fi

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

endpoint="${endpoint%/}"
# S3 canonical signing uses the URI-encoded path. curl signs the path exactly as
# given, while AWS CLI encodes '@' and other non-unreserved characters; the raw
# key produced SignatureDoesNotMatch for the '@v' path segment on R2.
encoded_key=$(python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe="/"))' "$probe_key")
url="$endpoint/$bucket/$encoded_key"

# curl_method METHOD URL OUT_STATUS OUT_BODY [EXTRA...]
#
# Returns non-zero only when curl produced no HTTP response; a completed HTTP
# response, including an error status, is a zero exit with the status text in
# OUT_STATUS. Diagnostic stderr is kept in a private file and never printed.
curl_method() {
  local method=$1 url=$2 out_status=$3 out_body=$4
  shift 4
  local args=(-sS -o "$out_body" -w '%{http_code}' -X "$method")
  if [[ -n ${netrc:-} && -f ${netrc:-} ]]; then
    args+=(--netrc-file "$netrc" --aws-sigv4 "aws:amz:auto:s3")
  fi
  args+=("$@")
  curl "${args[@]}" "$url" > "$out_status" 2> "$tmp/curl.stderr"
}

# read_object KEY -> sets status/body; returns non-zero on a transport failure.
read_object() {
  local out_status="$tmp/read.status" out_body="$tmp/read.body"
  rm -f "$out_status" "$out_body"
  if ! curl_method GET "$url" "$out_status" "$out_body"; then
    return 1
  fi
  printf '%s' "$(cat "$out_status")" > "$tmp/read.status.resolved"
  mv "$out_body" "$tmp/read.current"
  mv "$tmp/read.status.resolved" "$tmp/read.status.final"
  return 0
}

# read_fresh returns 0 and prints a one-line state for the current object:
#   present_identical SHA256
#   present_changed SHA256
#   missing
#   unreadable DETAIL
read_fresh() {
  if ! read_object; then
    printf 'unreadable no HTTP response (%s)\n' "$(head -c 200 "$tmp/curl.stderr" | tr '\n' ' ')"
    return 0
  fi
  local status
  status=$(cat "$tmp/read.status.final")
  if [[ $status == 200 ]]; then
    local hash
    hash=$(sha256sum "$tmp/read.current" | awk '{print $1}')
    if [[ $hash == "$staged_hash" ]]; then
      printf 'present_identical %s\n' "$hash"
    else
      printf 'present_changed %s\n' "$hash"
    fi
    return 0
  fi
  local code
  code=$(python3 - "$tmp/read.current" <<'PY'
import pathlib, re, sys
text = pathlib.Path(sys.argv[1]).read_text(errors="replace")
match = re.search(r"<Code>([^<]+)</Code>", text)
print(match.group(1) if match else "")
PY
)
  if [[ $status == 404 && $code == NoSuchKey ]]; then
    printf 'missing\n'
    return 0
  fi
  printf 'unreadable HTTP %s code %s\n' "$status" "${code:-unknown}"
}

# The service denial required by the R2 Bucket Lock contract.
REQUIRED_STATUS=409
REQUIRED_CODE=ObjectLockedByBucketPolicy

error_code() {
  python3 - "$1" <<'PY'
import pathlib, re, sys
text = pathlib.Path(sys.argv[1]).read_text(errors="replace")
match = re.search(r"<Code>([^<]+)</Code>", text)
print(match.group(1) if match else "")
PY
}

print_state() {
  local label=$1 state=$2
  echo "lock probe: $label -> $state"
}

# verify_or_repair_state STATE LABEL
#
# Fails the run with the observed state. When the state shows the object is
# missing, the exact staged bytes are re-created and verified first. Any other
# state is reported without inventing a missing object.
fatal_with_repair() {
  local state=$1 message=$2
  echo "lock probe: FAIL $message" >&2
  echo "lock probe: $state" >&2
  if [[ $state == present_identical* ]]; then
    echo "lock probe: the reviewed object bytes are unchanged" >&2
    exit 1
  fi
  if [[ $state == present_changed* ]]; then
    echo "lock probe: the object exists but no longer matches the reviewed bytes" >&2
    exit 1
  fi
  if [[ $state == unreadable* ]]; then
    echo "lock probe: the object state could not be determined; not treating this as missing or as success" >&2
    exit 1
  fi
  # state is exactly "missing": restore the exact staged bytes immediately.
  echo "lock probe: object is missing; restoring the exact staged bytes" >&2
  local attempt status body="$tmp/repair.body" statusfile="$tmp/repair.status"
  for attempt in 1 2 3; do
    rm -f "$body" "$statusfile"
    if curl_method PUT "$url" "$statusfile" "$body" \
      -H "Content-Type: $probe_content_type" \
      -H "Cache-Control: $probe_cache_control" \
      -H "x-amz-meta-sha256: $staged_hash" \
      -H 'If-None-Match: *' \
      --data-binary "@$staged_path"; then
      status=$(cat "$statusfile")
      if [[ $status == 200 || $status == 204 ]]; then
        local state_after
        state_after=$(read_fresh)
        if [[ $state_after == present_identical* ]]; then
          echo "lock probe: object restored and readback verified: ${state_after#present_identical }" >&2
          exit 1
        fi
        echo "lock probe: repaired object readback is $state_after" >&2
        exit 1
      fi
      if [[ $status == 412 || $status == 409 ]]; then
        # Another writer restored or holds the object; verify its bytes.
        local state_after
        state_after=$(read_fresh)
        echo "lock probe: repair create was refused with HTTP $status; readback is $state_after" >&2
        exit 1
      fi
    fi
    sleep 2
  done
  echo "lock probe: FAIL could not restore the missing object from the staged bytes" >&2
  exit 1
}

# 1. Pre-probe readback: the object must exist and match the reviewed bytes.
pre_state=$(read_fresh)
print_state "pre-probe readback" "$pre_state"
[[ $pre_state == present_identical* ]] || fatal_with_repair "$pre_state" "pre-probe readback did not show the reviewed object"

# 2. Unconditional overwrite of the identical reviewed bytes. There is no
# create-only condition, so only the service-side lock can deny this.
overwrite_status="$tmp/overwrite.status"
overwrite_body="$tmp/overwrite.body"
rm -f "$overwrite_status" "$overwrite_body"
if curl_method PUT "$url" "$overwrite_status" "$overwrite_body" \
  -H "Content-Type: $probe_content_type" \
  -H "Cache-Control: $probe_cache_control" \
  -H "x-amz-meta-sha256: $staged_hash" \
  --data-binary "@$staged_path"; then
  status=$(cat "$overwrite_status")
  code=$(error_code "$overwrite_body")
  if [[ $status == "$REQUIRED_STATUS" && $code == "$REQUIRED_CODE" ]]; then
    echo "lock probe: overwrite denied with HTTP $status $code; response: $(head -c 200 "$overwrite_body" | tr '\n' ' ')"
  else
    echo "lock probe: overwrite response was HTTP $status code ${code:-unknown}:" >&2
    head -c 400 "$overwrite_body" >&2 || true
    echo >&2
    state=$(read_fresh)
    fatal_with_repair "$state" "overwrite did not return the locked-bucket denial"
  fi
else
  echo "lock probe: overwrite produced no HTTP response (transport failure); the attempt may or may not have reached the service" >&2
  state=$(read_fresh)
  fatal_with_repair "$state" "overwrite outcome is uncertain"
fi

# 3. Delete of the object. The same exact denial is required.
delete_status="$tmp/delete.status"
delete_body="$tmp/delete.body"
rm -f "$delete_status" "$delete_body"
if curl_method DELETE "$url" "$delete_status" "$delete_body"; then
  status=$(cat "$delete_status")
  code=$(error_code "$delete_body")
  if [[ $status == "$REQUIRED_STATUS" && $code == "$REQUIRED_CODE" ]]; then
    echo "lock probe: delete denied with HTTP $status $code; response: $(head -c 200 "$delete_body" | tr '\n' ' ')"
  elif [[ $status == 200 || $status == 202 || $status == 204 ]]; then
    echo "lock probe: FAIL delete was accepted with HTTP $status:" >&2
    head -c 400 "$delete_body" >&2 || true
    echo >&2
    state=$(read_fresh)
    fatal_with_repair "$state" "the version lock did not deny delete"
  else
    echo "lock probe: delete response was HTTP $status code ${code:-unknown}:" >&2
    head -c 400 "$delete_body" >&2 || true
    echo >&2
    state=$(read_fresh)
    fatal_with_repair "$state" "delete did not return the locked-bucket denial"
  fi
else
  echo "lock probe: delete produced no HTTP response (transport failure); checking whether the object survived" >&2
  state=$(read_fresh)
  fatal_with_repair "$state" "delete outcome is uncertain"
fi

# 4. Final readback: the object must still be present with the reviewed bytes.
final_state=$(read_fresh)
print_state "final readback" "$final_state"
[[ $final_state == present_identical* ]] || fatal_with_repair "$final_state" "final readback did not show the reviewed object"
echo "lock probe: PASS key=$probe_key version=$version sha256=${final_state#present_identical } overwrite=denied(409 $REQUIRED_CODE) delete=denied(409 $REQUIRED_CODE) readback=unchanged"
