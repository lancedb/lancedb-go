#!/usr/bin/env bash
# Verify published companion modules through the public Go proxy and checksum
# database with fresh module caches and default sumdb verification.
#
# For every module in the manifest this script:
#   * fetches `https://go.lancedb.io/<module>?go-get=1` and checks the exact
#     three-field go-import meta;
#   * resolves `@version` through https://proxy.golang.org with a fresh
#     GOMODCACHE/GOCACHE and GOSUMDB=sum.golang.org, and requires the returned
#     Version, Sum and GoModSum to equal the manifest values;
#   * resolves the same version through `GOPROXY=direct` as a discovery-path
#     check; and
#   * byte-compares the payload inside the public zip with the staged payload.
#
# The script never reads credentials. Retries wait for proxy propagation within
# --timeout-seconds.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: verify-origin-public.sh --manifest PATH --origin DIR [--version VERSION]
       [--timeout-seconds N] [--interval-seconds N]

Manifest supplies the five module paths, the exact version and the h1 sums.
The origin tree supplies the staged payload bytes compared with the public zip.
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

work=$(mktemp -d)
trap 'chmod -R u+w "$work" 2>/dev/null || true; rm -rf "$work"' EXIT
deadline=$(( $(date +%s) + timeout ))

# The production defaults are the public origin, proxy and checksum database;
# the fixture test overrides them with a local server. Keeping the defaults
# means the real run has default sumdb verification enabled.
ORIGIN_BASE_URL=${ORIGIN_BASE_URL:-https://go.lancedb.io}
PROXY_URL=${PROXY_URL:-https://proxy.golang.org}
SUMDB=${SUMDB:-sum.golang.org}
DIRECT_PROBE=${DIRECT_PROBE:-1}

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

modules = {obj["module"]: obj["gomod_h1"] for obj in data["objects"] if obj["key"].endswith(".mod")}
rows = []
for obj in data["objects"]:
    if not obj.get("target") or not obj["key"].endswith(".zip"):
        continue
    rows.append("\t".join([obj["target"], obj["module"], obj["version"], obj["zip_h1"], modules[obj["module"]], str(origin / obj["key"])]))
rows.sort()
print("\n".join(rows))
PY

resolve() {
  # resolve MODULE VERSION MODE CACHE_ROOT
  local module=$1 module_version=$2 mode=$3 cache_root=$4
  local proxy
  if [[ $mode == direct ]]; then
    proxy=direct
  else
    proxy=$PROXY_URL
  fi
  mkdir -p "$cache_root/mod" "$cache_root/build"
  env -u GOPRIVATE -u GONOPROXY -u GONOSUMDB -u GONOSUMCHECK -u GOFLAGS \
    GOENV=off GOWORK=off GOTOOLCHAIN=local \
    GOPROXY="$proxy" GOSUMDB="$SUMDB" \
    GOMODCACHE="$cache_root/mod" GOCACHE="$cache_root/build" \
    go mod download -json "$module@$module_version" > "$cache_root/download.json" 2> "$cache_root/download.err"
}

check_download() {
  # check_download MODULE VERSION ZIP_H1 GOMOD_H1 CACHE_ROOT
  local module=$1 module_version=$2 zip_h1=$3 gomod_h1=$4 cache_root=$5
  python3 - "$cache_root/download.json" "$module" "$module_version" "$zip_h1" "$gomod_h1" <<'PY'
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

compare_payload() {
  # compare_payload MODULE VERSION PUBLIC_ZIP STAGED_ZIP
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

while IFS=$'\t' read -r target module module_version zip_h1 gomod_h1 staged_zip; do
  [[ -n $target ]] || continue
  echo "public verification: $module@$module_version ($target)"
  meta="content=\"$module mod https://go.lancedb.io/modules\""
  discovery_path=${module#go.lancedb.io/}
  if ! curl -fsS "$ORIGIN_BASE_URL/$discovery_path?go-get=1" | grep -F "$meta" >/dev/null; then
    echo "public verification: discovery meta missing for $module" >&2
    exit 1
  fi
  echo "  discovery html: ok"

  resolved=0
  last_error=""
  attempt=0
  while [[ $(date +%s) -lt $deadline ]]; do
    attempt=$((attempt + 1))
    cache_root="$work/$target/proxy-$attempt"
    if resolve "$module" "$module_version" proxy "$cache_root"; then
      if check_download "$module" "$module_version" "$zip_h1" "$gomod_h1" "$cache_root" > "$cache_root/check" 2>&1; then
        public_zip=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["Zip"])' "$cache_root/download.json")
        if compare_payload "$module" "$module_version" "$public_zip" "$staged_zip" > "$cache_root/compare" 2>&1; then
          resolved=1
          last_error="$cache_root"
          break
        fi
      fi
      last_error="$cache_root/check"
      cat "$last_error" >&2 || true
    else
      last_error="$cache_root/error"
      cat "$last_error" >&2 || true
    fi
    sleep "$interval"
  done
  if [[ $resolved != 1 ]]; then
    echo "public verification: FAIL $module@$module_version did not resolve through the public proxy" >&2
    [[ -n $last_error ]] && cat "$last_error" >&2 || true
    exit 1
  fi
  echo "  proxy and sumdb: ok"

  if [[ $DIRECT_PROBE != 1 ]]; then
    continue
  fi

  direct_cache="$work/$target/direct"
  if ! resolve "$module" "$module_version" direct "$direct_cache"; then
    echo "public verification: FAIL $module@$module_version did not resolve through discovery (GOPROXY=direct)" >&2
    cat "$direct_cache/error" >&2 || true
    exit 1
  fi
  if ! check_download "$module" "$module_version" "$zip_h1" "$gomod_h1" "$direct_cache" > "$direct_cache/check" 2>&1; then
    echo "public verification: FAIL $module@$module_version sums differ through direct resolution" >&2
    cat "$direct_cache/check" >&2 || true
    exit 1
  fi
  echo "  discovery path + sum.golang.org: ok"
done < "$work/modules.tsv"

echo "public verification: PASS"
