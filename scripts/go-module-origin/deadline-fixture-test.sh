#!/usr/bin/env bash
# Focused bounded fixture for the public-verification per-target deadline.
#
# A loopback server accepts a request, logs it, and holds the response without
# replying. The verifier must give up on that target at its budget, fail, and
# still start the next target with its own fresh budget. Both the discovery
# request and the proxy `go mod download` request are exercised.
#
# Usage: deadline-fixture-test.sh
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
GO=${GO:-go}
VERIFY_SCRIPT=${VERIFY_SCRIPT:-$repo_root/scripts/verify-origin-public.sh}
command -v "$GO" >/dev/null 2>&1 || { echo "deadline-fixture-test: go is required" >&2; exit 1; }
[[ -x $VERIFY_SCRIPT ]] || { echo "deadline-fixture-test: verifier is not executable: $VERIFY_SCRIPT" >&2; exit 1; }

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
python3 - "$origin" <<'PY'
import json
import pathlib
import sys

origin = pathlib.Path(sys.argv[1])
version = "v0.0.1"
prefix = "go.lancedb.io/lancedb-go/artifacts"
objects = []
for target in ("linux-amd64", "linux-arm64"):
    module = f"{prefix}/{target}"
    mod_key = f"modules/{module}/@v/{version}.mod"
    zip_key = f"modules/{module}/@v/{version}.zip"
    (origin / mod_key).parent.mkdir(parents=True, exist_ok=True)
    (origin / mod_key).write_text(f"module {module}\n\ngo 1.21.0\n", encoding="utf-8")
    (origin / zip_key).write_bytes(b"")
    objects.append({"module": module, "key": mod_key, "gomod_h1": "h1:fixture"})
    objects.append({"module": module, "key": zip_key, "target": target, "version": version, "zip_h1": "h1:fixture"})
(origin / "manifest.json").write_text(
    json.dumps(
        {
            "module_prefix": prefix,
            "version": version,
            "objects": objects,
        },
        indent=2,
    )
    + "\n",
    encoding="utf-8",
)
PY

fail() {
  echo "deadline-fixture-test: $*" >&2
  [[ -f ${log:-} ]] && cat "$log" >&2 || true
  [[ -f ${out:-} ]] && cat "$out" >&2 || true
  exit 1
}

run_phase() {
  local phase=$1 port_file log out port start elapsed rc
  port_file="$work/port-$phase"
  log="$work/server-$phase.log"
  out="$work/verify-$phase.log"
  python3 "$repo_root/scripts/go-module-origin/holding-server.py" \
    --mode "$phase" --port-file "$port_file" --log "$log" > "$work/server-$phase.stdout" 2>&1 &
  server_pid=$!
  for _ in $(seq 1 100); do
    [[ -s $port_file ]] && break
    sleep 0.1
  done
  [[ -s $port_file ]] || fail "$phase: holding server did not start"
  port=$(cat "$port_file")

  start=$(date +%s)
  set +e
  ORIGIN_BASE_URL="http://127.0.0.1:$port" \
  PROXY_URL="http://127.0.0.1:$port/modules" \
  SUMDB=off \
  DIRECT_PROBE=0 \
    timeout 30 "$VERIFY_SCRIPT" \
      --manifest "$origin/manifest.json" --origin "$origin" \
      --timeout-seconds 2 --interval-seconds 1 > "$out" 2>&1
  rc=$?
  set -e
  elapsed=$(( $(date +%s) - start ))
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  server_pid=""

  [[ $rc -eq 1 ]] || fail "$phase: verifier exited $rc, want 1 (its own bounded failure, not an outer timeout)"
  (( elapsed <= 20 )) || fail "$phase: verifier took ${elapsed}s for two 2s targets"
  grep -F 'public verification: go.lancedb.io/lancedb-go/artifacts/linux-amd64@v0.0.1' "$out" >/dev/null \
    || fail "$phase: first target was not attempted"
  grep -F 'public verification: go.lancedb.io/lancedb-go/artifacts/linux-arm64@v0.0.1' "$out" >/dev/null \
    || fail "$phase: later target did not get its own budget"
  grep -F 'did not resolve within the 2s per-target deadline' "$out" >/dev/null \
    || fail "$phase: bounded failure diagnostic missing"
  grep -F "$phase: received" "$log" >/dev/null \
    || fail "$phase: the fixture never established a request before holding it"
  grep -F 'artifacts/linux-amd64' "$log" >/dev/null \
    || fail "$phase: no request for the first target reached the holding server"
  grep -F 'artifacts/linux-arm64' "$log" >/dev/null \
    || fail "$phase: no request for the later target reached the holding server"
  echo "deadline fixture $phase: PASS (${elapsed}s, rc=$rc)"
}

run_phase discovery
run_phase proxy

echo "public verification deadline fixture: PASS"
