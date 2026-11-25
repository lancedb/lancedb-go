#!/usr/bin/env bash
# End-to-end fixture test for candidate generation and the origin tree.
#
# Builds a synthetic lancedb-c release repository and archive set (minimal, but
# parseable ELF/Mach-O/PE binaries), then runs the real generate-candidates.sh
# against it: tag verification, archive checksums, internal SHA256SUMS,
# candidate modules, origin protocol tree and the recorded identity. Runs the
# generation twice and byte-compares the result to prove determinism, then
# compiles one Go consumer per target module to prove the embed accessor works
# across GOOS/GOARCH with the Go 1.21 module floor.
#
# Usage: origin-fixture-test.sh OUT_DIR
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
out=${1:?usage: origin-fixture-test.sh OUT_DIR}
GO=${GO:-go}
command -v "$GO" >/dev/null 2>&1 || { echo "origin-fixture-test: go is required" >&2; exit 1; }

chmod -R u+w "$out" 2>/dev/null || true
rm -rf "$out"
mkdir -p "$out"

export GOWORK=off
export GOTOOLCHAIN=${GOTOOLCHAIN:-local}

work="$out/fixture"
release_repo="$work/release-repo"
release_files="$work/release"
mkdir -p "$release_repo" "$release_files"

# A local git repository with an annotated release tag proves the tag-to-commit
# verification without network access.
git -C "$release_repo" init -q -b main
git -C "$release_repo" config user.email fixture@example.com
git -C "$release_repo" config user.name Fixture
printf 'fixture\n' > "$release_repo/README.md"
git -C "$release_repo" add README.md
git -C "$release_repo" -c commit.gpgsign=false commit -q -m "fixture release"
commit=$(git -C "$release_repo" rev-parse HEAD)
git -C "$release_repo" tag -a v0.1.0-preview.5 -m "fixture release"
tag_object=$(git -C "$release_repo" rev-parse refs/tags/v0.1.0-preview.5)

python3 - "$release_files" "$commit" <<'PY'
import hashlib
import json
import pathlib
import sys
import tarfile

release_dir = pathlib.Path(sys.argv[1])
commit = sys.argv[2]
version = "0.1.0-preview.5"
tag = f"v{version}"

def sha256_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()

def elf(arch):
    machine = {"amd64": 0x3E, "arm64": 0xB7}[arch]
    data = bytearray(64)
    data[0:4] = b"\x7fELF"
    data[4:7] = bytes([2, 1, 1])
    data[16:18] = (3).to_bytes(2, "little")
    data[18:20] = machine.to_bytes(2, "little")
    data[20:24] = (1).to_bytes(4, "little")
    return bytes(data)

def macho():
    data = bytearray(32)
    data[0:4] = (0xFEEDFACF).to_bytes(4, "little")
    data[4:8] = (0x0100000C).to_bytes(4, "little")
    data[12:16] = (6).to_bytes(4, "little")
    return bytes(data)

def pe(arch):
    machine = {"amd64": 0x8664, "arm64": 0xAA64}[arch]
    header = 0x80
    optional = 0xF0
    data = bytearray(header + 4 + 20 + optional)
    data[0:2] = b"MZ"
    data[0x3C:0x40] = header.to_bytes(4, "little")
    data[header:header + 4] = b"PE\x00\x00"
    data[header + 4:header + 6] = machine.to_bytes(2, "little")
    data[header + 20:header + 22] = optional.to_bytes(2, "little")
    data[header + 22:header + 24] = (0x2000).to_bytes(2, "little")
    data[header + 24:header + 26] = (0x20B).to_bytes(2, "little")
    data[header + 24 + 108:header + 24 + 112] = (16).to_bytes(4, "little")
    return bytes(data)

targets = {
    "linux-amd64": {"library": "liblancedb.so", "member": "lib/liblancedb.so.0.1.0", "binary": elf("amd64"), "import": None},
    "linux-arm64": {"library": "liblancedb.so", "member": "lib/liblancedb.so.0.1.0", "binary": elf("arm64"), "import": None},
    "darwin-arm64": {"library": "liblancedb.dylib", "member": "lib/liblancedb.dylib", "binary": macho(), "import": None},
    "windows-amd64": {"library": "lancedb.dll", "member": "lib/lancedb.dll", "binary": pe("amd64"), "import": "lib/lancedb.dll.lib"},
    "windows-arm64": {"library": "lancedb.dll", "member": "lib/lancedb.dll", "binary": pe("arm64"), "import": "lib/lancedb.dll.lib"},
}

identity = {
    "version": tag,
    "time": "2026-11-25T00:00:00Z",
    "module_prefix": "go.lancedb.io/lancedb-go/artifacts",
    "base_url": "https://go.lancedb.io",
    "go_version": "1.21.0",
    "source_repo": str(pathlib.Path(sys.argv[1]).parent / "release-repo"),
    "source_release": tag,
    "source_commit": commit,
    "header_sha256": "",
    "license_sha256": "",
    "download_base_url": release_dir.as_uri(),
    "targets": {},
}

license_bytes = b"Apache License 2.0\n"
header_bytes = (
    "#ifndef LANCEDB_H\n#define LANCEDB_H\n"
    f'#define LANCEDB_VERSION_STRING "{version}"\n'
    "#endif\n"
).encode()
identity["header_sha256"] = hashlib.sha256(header_bytes).hexdigest()
identity["license_sha256"] = hashlib.sha256(license_bytes).hexdigest()

for target, spec in targets.items():
    root = release_dir / f"lancedb-c-{commit}-{target}"
    (root / "lib").mkdir(parents=True, exist_ok=True)
    (root / "include").mkdir(parents=True, exist_ok=True)
    (root / "LICENSE").write_bytes(license_bytes)
    (root / "include" / "lancedb.h").write_bytes(header_bytes)
    (root / spec["member"]).write_bytes(spec["binary"])
    if target.startswith("linux"):
        (root / spec["library"]).symlink_to(pathlib.PurePosixPath(spec["member"]).name)
    if spec["import"]:
        (root / spec["import"]).write_bytes(b"fixture import library\n")
    manifest = {
        "schemaVersion": 1,
        "name": "lancedb-c",
        "version": version,
        "target": target,
        "rustTarget": target,
        "commit": commit,
        "library": spec["library"],
        "include": "include/lancedb.h",
        "importLibrary": spec["import"],
        "license": "LICENSE",
    }
    (root / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    members = sorted(path for path in root.rglob("*") if path.is_file() and path.name != "SHA256SUMS")
    lines = [f"{sha256_file(path)}  {path.relative_to(root).as_posix()}" for path in members]
    (root / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")

    archive = release_dir / f"{root.name}.tar.gz"
    with tarfile.open(archive, "w:gz") as handle:
        handle.add(root, arcname=root.name)
    (release_dir / f"{archive.name}.sha256").write_text(
        f"{sha256_file(archive)}  {archive.name}\n", encoding="utf-8"
    )
    identity["targets"][target] = {
        "archive": archive.name,
        "archive_sha256": sha256_file(archive),
        "library": spec["member"],
        "library_sha256": sha256_file(root / spec["member"]),
    }

identity["source_tag_object"] = None
(pathlib.Path(release_dir).parent / "identity.json").write_text(
    json.dumps(identity, indent=2) + "\n", encoding="utf-8"
)
PY

identity="$work/identity.json"
# The local repository is lightweight-correct: fill the tag object/type that
# `git ls-remote` reports, without hardcoding them.
python3 - "$identity" "$release_repo" <<'PY'
import json
import pathlib
import subprocess
import sys

identity_path, repo = sys.argv[1], sys.argv[2]
identity = json.loads(pathlib.Path(identity_path).read_text(encoding="utf-8"))
output = subprocess.run(
    ["git", "ls-remote", repo, "refs/tags/" + identity["source_release"], "refs/tags/" + identity["source_release"] + "^{}"],
    text=True, capture_output=True, check=True,
).stdout
entries = {}
for line in output.splitlines():
    sha, ref = line.split("\t")
    entries[ref] = sha
identity["source_tag_object"] = entries["refs/tags/" + identity["source_release"]]
identity["source_tag_type"] = "annotated" if identity["source_tag_object"] != identity["source_commit"] else "lightweight"
pathlib.Path(identity_path).write_text(json.dumps(identity, indent=2, sort_keys=False) + "\n", encoding="utf-8")
PY

WORK_DIR="$out/generate-1" \
CANDIDATES_DIR="$out/candidates" \
DOWNLOADS_DIR="$out/downloads" \
ORIGIN_DIR="$out/origin" \
IDENTITY_OUT="$out/candidate-identity.json" \
IDENTITY="$identity" \
  "$repo_root/scripts/go-module-origin/generate-candidates.sh"

python3 "$repo_root/scripts/go-module-origin/verify-candidate-identity.py" "$out/candidate-identity.json" "$out/candidates"

ORIGIN_DIR="$out/origin" IDENTITY="$out/candidate-identity.json" "$repo_root/scripts/test-go-module-origin.sh"

# Determinism: regenerate into a second work directory and byte-compare the
# manifest and every module zip.
WORK_DIR="$out/generate-2" \
CANDIDATES_DIR="$out/candidates-2" \
DOWNLOADS_DIR="$out/downloads" \
ORIGIN_DIR="$out/origin-2" \
IDENTITY_OUT="$out/candidate-identity-2.json" \
IDENTITY="$identity" \
  "$repo_root/scripts/go-module-origin/generate-candidates.sh"
cmp "$out/origin/manifest.json" "$out/origin-2/manifest.json"
cmp "$out/candidate-identity.json" "$out/candidate-identity-2.json"
for target in linux-amd64 linux-arm64 darwin-arm64 windows-amd64 windows-arm64; do
  version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$out/origin/manifest.json")
  cmp "$out/origin/modules/go.lancedb.io/lancedb-go/artifacts/$target/@v/$version.zip" \
      "$out/origin-2/modules/go.lancedb.io/lancedb-go/artifacts/$target/@v/$version.zip"
done
echo "fixture determinism: ok"

# One consumer per target proves the embed accessor, the module Go floor and
# cross-target compilation with CGO_ENABLED=0 through the file proxy.
version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$out/origin/manifest.json")
consumer_root="$out/consumers"
mkdir -p "$consumer_root"
shared_modcache="$work/consumer-modcache"
for target in linux-amd64 linux-arm64 darwin-arm64 windows-amd64 windows-arm64; do
  goos=${target%-*}
  goarch=${target##*-}
  consumer="$consumer_root/$target"
  mkdir -p "$consumer"
  cat > "$consumer/go.mod" <<EOF
module fixture-consumer

go 1.21.0

require go.lancedb.io/lancedb-go/artifacts/$target $version
EOF
  cat > "$consumer/main.go" <<EOF
package main

import artifact "go.lancedb.io/lancedb-go/artifacts/$target"

func main() {
	if len(artifact.Payload) == 0 || artifact.Target == "" || artifact.LibraryName == "" {
		panic("empty artifact module")
	}
}
EOF
  (
    cd "$consumer"
    CGO_ENABLED=0 GOPROXY="file://$out/origin/modules" GOSUMDB=off GOFLAGS=-mod=mod \
      GOMODCACHE="$shared_modcache" GOCACHE="$work/consumer-gocache" GOWORK=off GOTOOLCHAIN=local \
      GOOS="$goos" GOARCH="$goarch" "$GO" build -o "$work/consumer-$target" ./...
  )
  echo "consumer $target: ok"
done

echo "origin fixture: PASS"
