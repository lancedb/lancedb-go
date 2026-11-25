#!/usr/bin/env bash
# Signed-path fixture for the lock probe.
#
# Reproduces the R2 signing defect locally: the probe must sign the URI-encoded
# object path (`%40v`), not the raw `@v` path. A local SigV4 verifier checks the
# signature with fixed test credentials, returns the staged .info for GET and
# HTTP 409 ObjectLockedByBucketPolicy for PUT/DELETE, so the real probe must
# pass. AWS CLI GET against the same verifier proves the compatible request
# path that already worked in the live publication.
#
# Usage: signed-path-fixture-test.sh
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
GO=${GO:-go}
PROBE_SCRIPT=${PROBE_SCRIPT:-$repo_root/scripts/verify-origin-locks.sh}
command -v "$GO" >/dev/null 2>&1 || { echo "signed-path-fixture-test: go is required" >&2; exit 1; }
[[ -x $PROBE_SCRIPT ]] || { echo "signed-path-fixture-test: probe is not executable: $PROBE_SCRIPT" >&2; exit 1; }

work=$(mktemp -d)
server_pid=""
cleanup() {
  [[ -n ${server_pid:-} ]] && kill "$server_pid" 2>/dev/null || true
  [[ -n ${server_pid:-} ]] && wait "$server_pid" 2>/dev/null || true
  chmod -R u+w "$work" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

origin="$work/origin"
mkdir -p "$origin"
probe_key='modules/go.lancedb.io/lancedb-go/artifacts/linux-amd64/@v/v0.0.1.info'
stage="$origin/$probe_key"
mkdir -p "$(dirname "$stage")"
printf '{"Version":"v0.0.1","Time":"2026-01-01T00:00:00Z"}\n' > "$stage"
python3 - "$origin" "$probe_key" "$stage" <<'PY'
import hashlib
import json
import pathlib
import sys

origin = pathlib.Path(sys.argv[1])
key = sys.argv[2]
stage = pathlib.Path(sys.argv[3])
raw = stage.read_bytes()
manifest = {
    "version": "v0.0.1",
    "module_prefix": "go.lancedb.io/lancedb-go/artifacts",
    "objects": [
        {
            "key": key,
            "kind": "v0.0.1.info",
            "module": "go.lancedb.io/lancedb-go/artifacts/linux-amd64",
            "version": "v0.0.1",
            "target": "linux-amd64",
            "sha256": hashlib.sha256(raw).hexdigest(),
            "size": len(raw),
            "content_type": "application/json",
            "cache_control": "public, max-age=31536000, immutable",
            "immutable": True,
        }
    ],
}
(origin / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
PY

port_file="$work/port"
log="$work/server.log"
python3 "$repo_root/scripts/go-module-origin/sigv4-verifier.py" \
  --port-file "$port_file" --log "$log" \
  --access-key TESTACCESSKEY --secret-key TESTSECRETKEY \
  --probe-key "$probe_key" --stage "$stage" > "$work/server.stdout" 2>&1 &
server_pid=$!
for _ in $(seq 1 100); do
  [[ -s $port_file ]] && break
  sleep 0.1
done
[[ -s $port_file ]] || { echo "signed-path-fixture-test: verifier did not start" >&2; cat "$work/server.stdout" >&2; exit 1; }
port=$(cat "$port_file")

export AWS_ACCESS_KEY_ID=TESTACCESSKEY
export AWS_SECRET_ACCESS_KEY=TESTSECRETKEY
export AWS_DEFAULT_REGION=auto
export AWS_EC2_METADATA_DISABLED=true

set +e
"$PROBE_SCRIPT" \
  --manifest "$origin/manifest.json" --bucket test-bucket \
  --endpoint "http://127.0.0.1:$port" --version v0.0.1 > "$work/probe.log" 2>&1
probe_rc=$?
set -e
if [[ $probe_rc -ne 0 ]] || ! grep -F 'lock probe: PASS' "$work/probe.log" >/dev/null; then
  echo "signed-path-fixture-test: probe failed with rc=$probe_rc" >&2
  cat "$work/probe.log" >&2
  cat "$log" >&2 || true
  exit 1
fi

# AWS CLI is the proven live publisher path; the same verifier must accept it.
aws s3api get-object --bucket test-bucket \
  --endpoint-url "http://127.0.0.1:$port" --key "$probe_key" "$work/awscli.body" >/dev/null 2>&1 \
  || { echo "signed-path-fixture-test: AWS CLI GET failed" >&2; exit 1; }
cmp "$work/awscli.body" "$stage"

grep -F 'GET ' "$log" | grep -F 'signature=valid' | grep -F '%40v' >/dev/null \
  || { echo "signed-path-fixture-test: GET was not a valid encoded-path signature" >&2; cat "$log" >&2; exit 1; }
grep -F 'PUT ' "$log" | grep -F 'signature=valid' | grep -F '%40v' >/dev/null \
  || { echo "signed-path-fixture-test: PUT was not a valid encoded-path signature" >&2; cat "$log" >&2; exit 1; }
grep -F 'DELETE ' "$log" | grep -F 'signature=valid' | grep -F '%40v' >/dev/null \
  || { echo "signed-path-fixture-test: DELETE was not a valid encoded-path signature" >&2; cat "$log" >&2; exit 1; }

kill "$server_pid" 2>/dev/null || true
wait "$server_pid" 2>/dev/null || true
server_pid=""

echo "signed-path fixture: PASS"
