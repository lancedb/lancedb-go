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
# Each target gets its own bounded deadline: a transient discovery or proxy
# failure retries until that deadline instead of consuming one shared window.
# The script never reads credentials.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify-origin-public.sh --manifest PATH --origin DIR [--version VERSION]
       [--timeout-seconds N] [--interval-seconds N]

Manifest supplies the five module paths, the exact version and the h1 sums.
The origin tree supplies the staged payload bytes compared with the public zip.

--timeout-seconds is the per-target deadline for discovery, proxy resolution
and the direct discovery-path resolution (default 900). --interval-seconds is
the wait between attempts (default 20).

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

# resolve MODULE VERSION MODE CACHE_ROOT
# Writes stdout.json and stderr.log inside CACHE_ROOT. Returns non-zero when
# the command could not produce a download result at all.
resolve() {
  local module=$1 module_version=$2 mode=$3 cache_root=$4
  local proxy
  if [[ $mode == direct ]]; then
    proxy=direct
  else
    proxy=$PROXY_URL
  fi
  mkdir -p "$cache_root/mod" "$cache_root/build"
  rm -f "$probe_dir/go.sum"
  env -u GOPRIVATE -u GONOPROXY -u GONOSUMDB -u GONOSUMCHECK -u GOFLAGS \
    GOENV=off GOWORK=off GOTOOLCHAIN=local \
    GOPROXY="$proxy" GOSUMDB="$SUMDB" \
    GOMODCACHE="$cache_root/mod" GOCACHE="$cache_root/build" \
    go -C "$probe_dir" mod download -json "$module@$module_version" \
    > "$cache_root/stdout.json" 2> "$cache_root/stderr.log" \
    && return 0
  return 1
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

resolve_until() {
  # resolve_until MODULE VERSION MODE LABEL DEADLINE CACHE_PREFIX ZIP_H1 GOMOD_H1
  # Prints the successful cache root. On timeout, prints the last diagnostics
  # to stderr and returns 1.
  local module=$1 module_version=$2 mode=$3 label=$4 deadline=$5 cache_prefix=$6 zip_h1=$7 gomod_h1=$8
  local attempt=0 cache_root
  while [[ $(date +%s) -lt $deadline ]]; do
    attempt=$((attempt + 1))
    cache_root="$cache_prefix-$attempt"
    if resolve "$module" "$module_version" "$mode" "$cache_root" \
      && check_download "$module" "$module_version" "$zip_h1" "$gomod_h1" "$cache_root" > "$cache_root/check.log" 2>&1; then
      printf '%s\n' "$cache_root"
      return 0
    fi
    echo "  $label attempt $attempt: not ready" >&2
    if [[ -f $cache_root/stderr.log ]]; then
      tail -n 3 "$cache_root/stderr.log" >&2 || true
    fi
    if [[ -f $cache_root/check.log ]]; then
      cat "$cache_root/check.log" >&2 || true
    fi
    sleep "$interval"
  done
  echo "  $label did not succeed within the per-target deadline" >&2
  return 1
}

status=0
while IFS=$'\t' read -r target module module_version zip_h1 gomod_h1 staged_zip; do
  [[ -n $target ]] || continue
  echo "public verification: $module@$module_version ($target)"
  discovery_path=${module#go.lancedb.io/}
  meta="content=\"$module mod https://go.lancedb.io/modules\""
  deadline=$(( $(date +%s) + timeout ))

  # Discovery and proxy/sumdb resolution share the target's deadline; both are
  # retried so a transient proxy or origin propagation pause does not fail the
  # target immediately.
  proxy_cache=""
  attempt=0
  while [[ $(date +%s) -lt $deadline ]]; do
    attempt=$((attempt + 1))
    if ! curl -fsS "$ORIGIN_BASE_URL/$discovery_path?go-get=1" | grep -F "$meta" >/dev/null; then
      echo "  discovery attempt $attempt: meta not ready" >&2
      sleep "$interval"
      continue
    fi
    if proxy_cache=$(resolve_until "$module" "$module_version" proxy "proxy + sumdb" "$deadline" "$work/$target/proxy" "$zip_h1" "$gomod_h1"); then
      break
    fi
  done
  if [[ -z $proxy_cache ]]; then
    echo "public verification: FAIL $module@$module_version did not resolve through the proxy within the deadline" >&2
    status=1
    continue
  fi
  public_zip=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["Zip"])' "$proxy_cache/stdout.json")
  if ! compare_payload "$module" "$module_version" "$public_zip" "$staged_zip"; then
    echo "public verification: FAIL $module@$module_version public payload differs from the staged payload" >&2
    status=1
    continue
  fi
  echo "  discovery html: ok"
  echo "  proxy and sumdb: ok"

  if [[ $DIRECT_PROBE == 1 ]]; then
    if ! resolve_until "$module" "$module_version" direct "discovery path + sumdb" "$deadline" "$work/$target/direct" "$zip_h1" "$gomod_h1" >/dev/null; then
      echo "public verification: FAIL $module@$module_version did not resolve through discovery within the deadline" >&2
      status=1
      continue
    fi
    echo "  discovery path + sumdb: ok"
  fi
done < "$work/modules.tsv"

if [[ $status != 0 ]]; then
  echo "public verification: FAIL" >&2
  exit 1
fi
echo "public verification: PASS"
