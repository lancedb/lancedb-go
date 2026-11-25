#!/usr/bin/env bash
# Verify a generated origin tree with the real Go module protocol, a local HTTP
# discovery and proxy server, and the recorded library hashes.
#
# Usage:
#   ORIGIN_DIR=<origin> IDENTITY=<recorded identity.json> scripts/test-go-module-origin.sh
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
origin=${ORIGIN_DIR:?ORIGIN_DIR must point at the generated origin tree}
manifest="$origin/manifest.json"
identity=${IDENTITY:-}
GO=${GO:-go}
command -v "$GO" >/dev/null 2>&1 || { echo "test-go-module-origin: required command not found: $GO" >&2; exit 1; }
command -v zstd >/dev/null 2>&1 || { echo "test-go-module-origin: required command not found: zstd" >&2; exit 1; }

python3 - "$manifest" "$identity" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
identity_path = sys.argv[2]
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
assert manifest["module_prefix"] == "go.lancedb.io/lancedb-go/artifacts", manifest["module_prefix"]
assert manifest["base_url"] == "https://go.lancedb.io", manifest["base_url"]
assert manifest["go_version"] == "1.21.0", manifest["go_version"]
assert manifest["version"].startswith("v"), manifest["version"]
objects = manifest["objects"]
assert len(objects) == 25, len(objects)
keys = {obj["key"] for obj in objects}
for target in ("linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64"):
    assert f"lancedb-go/artifacts/{target}" in keys, target
    module = f"go.lancedb.io/lancedb-go/artifacts/{target}"
    for suffix in ("list", f"{manifest['version']}.info", f"{manifest['version']}.mod", f"{manifest['version']}.zip"):
        assert f"modules/{module}/@v/{suffix}" in keys, (target, suffix)
if identity_path:
    identity = json.loads(pathlib.Path(identity_path).read_text(encoding="utf-8"))
    digest = hashlib.sha256(manifest_path.read_bytes()).hexdigest()
    assert identity["manifest_sha256"] == digest, (identity["manifest_sha256"], digest)
    assert identity["version"] == manifest["version"], (identity["version"], manifest["version"])
    assert identity["source_commit"] == manifest["source_commit"], (identity["source_commit"], manifest["source_commit"])
    assert identity["source_release"] == manifest["source_release"], (identity["source_release"], manifest["source_release"])
    for target in identity["targets"]:
        assert target in {obj["target"] for obj in objects}, target
print("manifest and identity: ok")
PY

work=$(mktemp -d)
server_pid=""
cleanup() {
  [[ -n ${server_pid:-} ]] && kill "$server_pid" 2>/dev/null || true
  chmod -R u+w "$work" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

python3 - "$manifest" > "$work/modules.tsv" <<'PY'
import json
import sys
manifest = json.load(open(sys.argv[1], encoding="utf-8"))
mods = {obj["module"]: obj["gomod_h1"] for obj in manifest["objects"] if obj["key"].endswith(".mod")}
for obj in manifest["objects"]:
    if obj["key"].endswith(".zip"):
        print("\t".join([obj["target"], obj["module"], obj["version"], obj["zip_h1"], mods[obj["module"]], obj["key"]]))
PY

cache="$work/file-modcache"
while IFS=$'\t' read -r target module version zip_h1 gomod_h1 key; do
  [[ -n $target ]] || continue
  json="$work/file-$target.json"
  GOPROXY="file://$origin/modules" GOSUMDB=off GOMODCACHE="$cache" GOCACHE="$work/gocache" \
    GOFLAGS=-mod=mod GOWORK=off GOTOOLCHAIN=local \
    "$GO" mod download -json "$module@$version" > "$json"
  python3 - "$json" "$zip_h1" "$gomod_h1" "$module" <<'PY'
import json
import sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
assert not data.get("Error"), data
assert data["Sum"] == sys.argv[2], (sys.argv[4], data["Sum"], sys.argv[2])
assert data["GoModSum"] == sys.argv[3], (sys.argv[4], data["GoModSum"], sys.argv[3])
print(sys.argv[4], "file-proxy h1: ok")
PY
done < "$work/modules.tsv"

if [[ -n $identity ]]; then
  python3 - "$manifest" "$identity" > "$work/payloads.tsv" <<'PY'
import json
import sys
manifest = json.load(open(sys.argv[1], encoding="utf-8"))
identity = json.load(open(sys.argv[2], encoding="utf-8"))
for obj in manifest["objects"]:
    if obj["key"].endswith(".zip"):
        expected = identity["targets"][obj["target"]]["library_sha256"]
        print("\t".join([obj["target"], obj["module"], obj["version"], expected, obj["key"]]))
PY
  while IFS=$'\t' read -r target module version library_sha key; do
    [[ -n $target ]] || continue
    zip="$origin/$key"
    payload_name=$(python3 -c 'import sys,zipfile; archive=zipfile.ZipFile(sys.argv[1]); names=[n for n in archive.namelist() if n.endswith(".zst")]; assert len(names)==1, names; print(names[0])' "$zip")
    unzip -p "$zip" "$payload_name" > "$work/$target.zst"
    zstd -q -d -f "$work/$target.zst" -o "$work/$target.lib"
    actual=$(sha256sum "$work/$target.lib" | awk '{print $1}')
    [[ $actual == "$library_sha" ]] || { echo "$target: library hash $actual, want $library_sha" >&2; exit 1; }
    echo "$target native library: ok"
  done < "$work/payloads.tsv"
fi

server_pid=""
python3 "$repo_root/scripts/go-module-origin/test-server.py" --root "$origin" --port 18888 > "$work/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
  if curl -fsS -o /dev/null "http://127.0.0.1:18888/lancedb-go/artifacts/linux-amd64?go-get=1" 2>/dev/null; then
    break
  fi
  sleep 0.2
done
while IFS=$'\t' read -r target module version zip_h1 gomod_h1 key; do
  [[ -n $target ]] || continue
  meta="content=\"$module mod https://go.lancedb.io/modules\""
  discovery_path=${module#go.lancedb.io/}
  curl -fsS "http://127.0.0.1:18888/$discovery_path?go-get=1" | grep -F "$meta" >/dev/null
  curl -fsSI "http://127.0.0.1:18888/$discovery_path?go-get=1" | grep -qi '^Content-type: text/html'
done < "$work/modules.tsv"
first_module=$(cut -f2 "$work/modules.tsv" | head -1)
first_version=$(cut -f3 "$work/modules.tsv" | head -1)
curl -fsS "http://127.0.0.1:18888/modules/$first_module/@v/list" | grep -Fx "$first_version" >/dev/null
http_cache="$work/http-modcache"
GOPROXY="http://127.0.0.1:18888/modules" GOSUMDB=off GOMODCACHE="$http_cache" GOCACHE="$work/http-gocache" \
  GOFLAGS=-mod=mod GOWORK=off GOTOOLCHAIN=local \
  "$GO" mod download -json "$first_module@$first_version" > "$work/http.json"
python3 - "$work/http.json" <<'PY'
import json
import sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
assert not data.get("Error"), data
assert data.get("Sum", "").startswith("h1:"), data
print("http proxy and discovery: ok")
PY
kill "$server_pid" 2>/dev/null || true
server_pid=""

echo "origin verification: PASS"
