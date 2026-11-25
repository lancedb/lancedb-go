#!/usr/bin/env bash
# Verify published companion modules through the public Go proxy and checksum
# database with fresh module caches and default sumdb verification.
#
# For every module in the manifest this script:
#   * fetches `https://go.lancedb.io/<module>?go-get=1` and checks the exact
#     three-field go-import meta;
#   * resolves `@version` through https://proxy.golang.org from a fresh
#     temporary probe module (no root go.mod/go.sum or workspace influence)
#     with fresh GOMODCACHE/GOCACHE and GOSUMDB=sum.golang.org, and requires
#     the returned Version, Sum and GoModSum to equal the manifest values
#     exactly;
#   * resolves the same version through `GOPROXY=direct` as a discovery-path
#     check; and
#   * byte-compares the payload inside the public zip with the staged payload.
#
# Each target owns one monotonic, fractional deadline. Every network attempt
# (discovery curl, proxy `go mod download`, direct `go mod download`) receives
# only the remaining milliseconds of that budget, and every retry sleep is
# clipped to the remaining time. A failed target does not consume another
# target's budget. curl and go run under `timeout` with a one-second
# TERM-to-KILL grace for the immediate command; this is not a general guarantee
# for arbitrary detached descendants.
#
# The script never reads credentials.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify-origin-public.sh --manifest PATH --origin DIR [--version VERSION]
       [--timeout-seconds N] [--interval-seconds N]

Manifest supplies the five module paths, the exact version and the h1 sums.
The origin tree supplies the staged payload bytes compared with the public zip.

--timeout-seconds is the total budget for one target, covering discovery,
proxy resolution, the direct discovery-path resolution, and retry waits
(default 900). The budget keeps fractional monotonic time; every attempt gets
only the remaining time. --interval-seconds is the wait between attempts
(default 20), clipped to the remaining budget. curl and go are bounded with a
one-second TERM-to-KILL grace for the immediate command.

Environment overrides, for fixtures only:
  ORIGIN_BASE_URL  discovery base (default https://go.lancedb.io)
  PROXY_URL        proxy base (default https://proxy.golang.org)
  SUMDB            checksum database (default sum.golang.org)
  DIRECT_PROBE     1 to run GOPROXY=direct, 0 to skip (default 1)
EOF
}

manifest=""
origin=""
version=""
timeout=900
interval=20
while [[ $# -gt 0 ]]; do
  case "$1" in
  --manifest) manifest=$2; shift 2 ;;
  --origin) origin=$2; shift 2 ;;
  --version) version=$2; shift 2 ;;
  --timeout-seconds) timeout=$2; shift 2 ;;
  --interval-seconds) interval=$2; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n $manifest && -n $origin ]] || { usage >&2; exit 2; }
[[ -f $manifest ]] || { echo "missing manifest: $manifest" >&2; exit 1; }
[[ -d $origin ]] || { echo "missing origin tree: $origin" >&2; exit 1; }
command -v go >/dev/null || { echo "go is required" >&2; exit 1; }
command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 is required" >&2; exit 1; }
command -v timeout >/dev/null || { echo "timeout (coreutils) is required" >&2; exit 1; }

ORIGIN_BASE_URL=${ORIGIN_BASE_URL:-https://go.lancedb.io}
PROXY_URL=${PROXY_URL:-https://proxy.golang.org}
SUMDB=${SUMDB:-sum.golang.org}
DIRECT_PROBE=${DIRECT_PROBE:-1}

# The public origin must be verified with the checksum database enabled. The
# fixture test overrides both the base URL and the checksum database together.
if [[ $SUMDB == off && $ORIGIN_BASE_URL == https://go.lancedb.io ]]; then
  echo "verify-origin-public: refusing to verify the public origin with the checksum database disabled" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'chmod -R u+w "$work" 2>/dev/null || true; rm -rf "$work"' EXIT

# The probe runs from its own tiny module so the repository's go.mod, go.sum,
# GOWORK or GOFLAGS cannot satisfy the resolution from local state.
probe_dir="$work/probe"
mkdir -p "$probe_dir"
printf 'module public-origin-probe\n\ngo 1.21.0\n' > "$probe_dir/go.mod"

# One monotonic deadline per target. Python is already required by this script,
# so time.monotonic_ns() keeps fractional elapsed time without adding a new
# dependency. Every stage recomputes the remaining milliseconds instead of
# rounding to whole seconds.
monotonic_ns() {
  python3 -c 'import time; print(time.monotonic_ns())'
}

ms_to_seconds() {
  local milliseconds=$1
  printf '%d.%03d' $(( milliseconds / 1000 )) $(( milliseconds % 1000 ))
}

# remaining_ms DEADLINE_NS prints the remaining milliseconds. It returns 1
# when the target budget is exhausted.
remaining_ms() {
  local deadline_ns=$1 current_ns remaining
  current_ns=$(monotonic_ns)
  remaining=$(( (deadline_ns - current_ns) / 1000000 ))
  if (( remaining <= 0 )); then
    return 1
  fi
  printf '%s\n' "$remaining"
}

# bounded SECONDS COMMAND... runs one network attempt under a hard timeout.
# curl and go are the only commands this covers; timeout signals their
# immediate process group and uses a one-second TERM-to-KILL grace. This is not
# a general guarantee for arbitrary detached descendants. The go case logs
# whether the exact command from this attempt actually terminated.
bounded() {
  local seconds=$1
  shift
  timeout --signal=TERM --kill-after=1s "${seconds}s" "$@"
}

# bounded_sleep DEADLINE_NS INTERVAL sleeps for the retry interval, clipped to
# the remaining fractional budget, and does nothing once it is exhausted.
bounded_sleep() {
  local deadline_ns=$1 wait_seconds=$2 remaining limit
  if ! remaining=$(remaining_ms "$deadline_ns"); then
    return 0
  fi
  limit=$(( wait_seconds * 1000 ))
  if (( remaining > limit )); then
    remaining=$limit
  fi
  sleep "$(ms_to_seconds "$remaining")"
}

# check_go_terminated MODULE VERSION MODE logs whether the exact go command
# from this attempt is gone after a timeout. The pattern is scoped to this
# run's unique probe directory, module and version.
check_go_terminated() {
  local module=$1 version=$2 mode=$3 pattern
  pattern="go -C $probe_dir mod download -json $module@$version"
  if ! command -v pgrep >/dev/null 2>&1; then
    echo "  $mode: timeout reported the immediate go command terminated (pgrep unavailable for confirmation)" >&2
    return 0
  fi
  if pgrep -f "$pattern" >/dev/null 2>&1; then
    echo "  $mode: go process still present after timeout; sending TERM then KILL" >&2
    pkill -TERM -f "$pattern" 2>/dev/null || true
    sleep 0.2
    if pgrep -f "$pattern" >/dev/null 2>&1; then
      pkill -KILL -f "$pattern" 2>/dev/null || true
      echo "  $mode: go process required SIGKILL after the timeout grace" >&2
    else
      echo "  $mode: go process terminated after TERM" >&2
    fi
    return 0
  fi
  echo "  $mode: go process terminated within the timeout grace" >&2
}

python3 - "$manifest" "$origin" "$version" > "$work/modules.tsv" <<'PY'
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
origin = pathlib.Path(sys.argv[2]).resolve()
requested = sys.argv[3]
data = json.loads(manifest_path.read_text(encoding="utf-8"))
assert data.get("module_prefix") == "go.lancedb.io/lancedb-go/artifacts", data.get("module_prefix")
full_version = data["version"]
if requested:
    assert requested == full_version, (requested, full_version)

mod_h1 = {obj["module"]: obj["gomod_h1"] for obj in data["objects"] if obj["key"].endswith(".mod")}
rows = []
for obj in data["objects"]:
    if not obj.get("target") or not obj["key"].endswith(".zip"):
        continue
    staged_zip = origin / obj["key"]
    assert staged_zip.is_file(), staged_zip
    rows.append("\t".join([obj["target"], obj["module"], obj["version"], obj["zip_h1"], mod_h1[obj["module"]], str(staged_zip)]))
rows.sort()
print("\n".join(rows))
PY

# resolve MODULE VERSION MODE CACHE_ROOT DEADLINE_NS
# Writes stdout.json and stderr.log inside CACHE_ROOT. The go command gets the
# fractional remaining target budget. Returns non-zero when the command failed
# or the budget ran out.
resolve() {
  local module=$1 module_version=$2 mode=$3 cache_root=$4 deadline_ns=$5
  local budget_ms remaining proxy rc
  if ! budget_ms=$(remaining_ms "$deadline_ns"); then
    echo "  $mode attempt skipped: target budget exhausted" >&2
    return 1
  fi
  remaining=$(ms_to_seconds "$budget_ms")
  if [[ $mode == direct ]]; then
    proxy=direct
  else
    proxy=$PROXY_URL
  fi
  mkdir -p "$cache_root/mod" "$cache_root/build"
  rm -f "$probe_dir/go.sum"
  if bounded "$remaining" env -u GOPRIVATE -u GONOPROXY -u GONOSUMDB -u GONOSUMCHECK -u GOFLAGS \
    GOENV=off GOWORK=off GOTOOLCHAIN=local \
    GOPROXY="$proxy" GOSUMDB="$SUMDB" \
    GOMODCACHE="$cache_root/mod" GOCACHE="$cache_root/build" \
    go -C "$probe_dir" mod download -json "$module@$module_version" \
    > "$cache_root/stdout.json" 2> "$cache_root/stderr.log"; then
    return 0
  else
    rc=$?
  fi
  if (( rc == 124 )); then
    echo "  $mode attempt timed out after ${remaining}s (one-second TERM/KILL grace)" >&2
    check_go_terminated "$module" "$module_version" "$mode"
  fi
  return "$rc"
}

# check_download MODULE VERSION ZIP_H1 GOMOD_H1 CACHE_ROOT
check_download() {
  local module=$1 module_version=$2 zip_h1=$3 gomod_h1=$4 cache_root=$5
  python3 - "$cache_root/stdout.json" "$module" "$module_version" "$zip_h1" "$gomod_h1" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
module, version, zip_h1, gomod_h1 = sys.argv[2:6]
data = json.loads(path.read_text(encoding="utf-8"))
if data.get("Error"):
    raise SystemExit(f"{module}: {data['Error']}")
if data.get("Version") != version:
    raise SystemExit(f"{module}: resolved {data.get('Version')!r}, want {version!r}")
if data.get("Sum") != zip_h1:
    raise SystemExit(f"{module}: zip sum {data.get('Sum')!r}, want {zip_h1!r}")
if data.get("GoModSum") != gomod_h1:
    raise SystemExit(f"{module}: go.mod sum {data.get('GoModSum')!r}, want {gomod_h1!r}")
zip_path = data.get("Zip")
if not zip_path or not pathlib.Path(zip_path).is_file():
    raise SystemExit(f"{module}: download returned no zip: {zip_path!r}")
print(f"{module}: sums ok")
PY
}

# compare_payload MODULE VERSION PUBLIC_ZIP STAGED_ZIP
compare_payload() {
  local module=$1 module_version=$2 public_zip=$3 staged_zip=$4
  python3 - "$module" "$module_version" "$public_zip" "$staged_zip" <<'PY'
import hashlib
import sys
import zipfile

module, version, public_zip, staged_zip = sys.argv[1:5]
prefix = f"{module}@{version}/"

def payload_bytes(zip_path):
    with zipfile.ZipFile(zip_path) as archive:
        payloads = [name for name in archive.namelist() if name.endswith(".zst") and name.startswith(prefix)]
        if len(payloads) != 1:
            raise SystemExit(f"{zip_path}: {len(payloads)} payload members")
        return archive.read(payloads[0])

public = payload_bytes(public_zip)
staged = payload_bytes(staged_zip)
if hashlib.sha256(public).hexdigest() != hashlib.sha256(staged).hexdigest():
    raise SystemExit(f"{module}: public payload bytes differ from the staged payload")
print(f"{module}: payload bytes ok")
PY
}

status=0
while IFS=$'\t' read -r target module module_version zip_h1 gomod_h1 staged_zip; do
  [[ -n $target ]] || continue
  echo "public verification: $module@$module_version ($target)"
  discovery_path=${module#go.lancedb.io/}
  meta="content=\"$module mod https://go.lancedb.io/modules\""
  deadline_ns=$(( $(monotonic_ns) + timeout * 1000000000 ))
  attempt=0
  success=0

  while :; do
    if ! budget_ms=$(remaining_ms "$deadline_ns"); then
      break
    fi
    attempt=$((attempt + 1))
    remaining=$(ms_to_seconds "$budget_ms")
    echo "  attempt $attempt: remaining ${remaining}s"

    # Discovery is bounded by the remaining fractional budget.
    connect_ms=$budget_ms
    if (( connect_ms > 10000 )); then
      connect_ms=10000
    fi
    connect_timeout=$(ms_to_seconds "$connect_ms")
    if ! bounded "$remaining" curl -fsS --max-time "$remaining" --connect-timeout "$connect_timeout" \
      "$ORIGIN_BASE_URL/$discovery_path?go-get=1" > "$work/discovery.body" 2> "$work/discovery.stderr" \
      || ! grep -F "$meta" "$work/discovery.body" >/dev/null; then
      echo "  discovery attempt $attempt failed or timed out" >&2
      if [[ -f $work/discovery.stderr ]]; then
        tail -n 3 "$work/discovery.stderr" >&2 || true
      fi
      bounded_sleep "$deadline_ns" "$interval"
      continue
    fi

    proxy_cache="$work/$target/proxy-$attempt"
    if resolve "$module" "$module_version" proxy "$proxy_cache" "$deadline_ns" \
      && check_download "$module" "$module_version" "$zip_h1" "$gomod_h1" "$proxy_cache" \
        > "$proxy_cache/check.log" 2>&1; then
      public_zip=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["Zip"])' "$proxy_cache/stdout.json")
      if ! compare_payload "$module" "$module_version" "$public_zip" "$staged_zip" \
        > "$proxy_cache/compare.log" 2>&1; then
        echo "  proxy attempt $attempt: public payload differs from the staged payload" >&2
        cat "$proxy_cache/compare.log" >&2 || true
        bounded_sleep "$deadline_ns" "$interval"
        continue
      fi

      if [[ $DIRECT_PROBE == 1 ]]; then
        direct_cache="$work/$target/direct-$attempt"
        if ! resolve "$module" "$module_version" direct "$direct_cache" "$deadline_ns" \
          || ! check_download "$module" "$module_version" "$zip_h1" "$gomod_h1" "$direct_cache" \
            > "$direct_cache/check.log" 2>&1; then
          echo "  direct attempt $attempt failed or timed out" >&2
          if [[ -f $direct_cache/stderr.log ]]; then
            tail -n 5 "$direct_cache/stderr.log" >&2 || true
          fi
          if [[ -f $direct_cache/check.log ]]; then
            cat "$direct_cache/check.log" >&2 || true
          fi
          bounded_sleep "$deadline_ns" "$interval"
          continue
        fi
      fi

      echo "  discovery html: ok"
      echo "  proxy and sumdb: ok"
      if [[ $DIRECT_PROBE == 1 ]]; then
        echo "  discovery path + sumdb: ok"
      fi
      success=1
      break
    fi

    echo "  proxy attempt $attempt failed or timed out" >&2
    if [[ -f $proxy_cache/stderr.log ]]; then
      tail -n 5 "$proxy_cache/stderr.log" >&2 || true
    fi
    if [[ -f $proxy_cache/check.log ]]; then
      cat "$proxy_cache/check.log" >&2 || true
    fi
    bounded_sleep "$deadline_ns" "$interval"
  done

  if [[ $success != 1 ]]; then
    echo "public verification: FAIL $module@$module_version did not resolve within the ${timeout}s per-target deadline" >&2
    status=1
  fi
done < "$work/modules.tsv"

if [[ $status != 0 ]]; then
  echo "public verification: FAIL" >&2
  exit 1
fi
echo "public verification: PASS"
