#!/usr/bin/env python3
"""Fetch and verify the exact lancedb-c release archives for one publication.

The identity file pins the upstream tag, commit, per-target archive checksum,
library checksum and header/license identity. This command verifies the
published tag, the published per-asset checksum, the archive's internal
SHA256SUMS and manifest, then extracts the shared library and license into a
layout the candidate generator consumes. It never publishes anything.

Downloads are bounded: every archive has a hard size limit, transports retry a
bounded number of times, and only the exact asset names in the identity file
are requested.

Usage:
  fetch-release.py --identity PATH --downloads DIR --out DIR
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
from typing import Dict, List

MAX_ARCHIVE_BYTES_DEFAULT = 400 * 1024 * 1024
MAX_TOTAL_BYTES_DEFAULT = 2 * 1024 * 1024 * 1024
DOWNLOAD_ATTEMPTS = 3

LIBRARY_NAMES = {
    "linux-amd64": "liblancedb.so",
    "linux-arm64": "liblancedb.so",
    "darwin-arm64": "liblancedb.dylib",
    "windows-amd64": "lancedb.dll",
    "windows-arm64": "lancedb.dll",
}


def fail(message: str) -> "None":
    raise SystemExit(f"fetch-release: {message}")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def is_hex(value: str, size: int) -> bool:
    if len(value) != size:
        return False
    return all(c in "0123456789abcdef" for c in value)


def tag_coordinates(source_repo: str, release: str) -> Dict[str, str]:
    remote = source_repo
    if "://" not in remote and not remote.startswith((".", "/")):
        remote = "https://github.com/" + remote
    ref = f"refs/tags/{release}"
    peeled = ref + "^{}"
    completed = subprocess.run(
        ["git", "ls-remote", remote, ref, peeled],
        text=True,
        capture_output=True,
    )
    if completed.returncode != 0:
        fail(f"git ls-remote {remote} {ref}: {completed.stderr.strip()}")
    entries: Dict[str, str] = {}
    for line in completed.stdout.splitlines():
        fields = line.split("\t")
        if len(fields) != 2:
            continue
        entries[fields[1]] = fields[0]
    if ref not in entries:
        fail(f"release tag {release} does not exist in {remote}")
    tag_object = entries[ref]
    if peeled in entries:
        return {"tag_object": tag_object, "commit": entries[peeled], "tag_type": "annotated"}
    return {"tag_object": tag_object, "commit": tag_object, "tag_type": "lightweight"}


def download(url: str, destination: pathlib.Path, max_bytes: int) -> None:
    if destination.is_file():
        if destination.stat().st_size > max_bytes:
            fail(f"cached {destination.name} exceeds the size bound")
        return
    destination.parent.mkdir(parents=True, exist_ok=True)
    last_error: Exception | None = None
    for attempt in range(1, DOWNLOAD_ATTEMPTS + 1):
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "lancedb-go-go-module-origin"})
            with urllib.request.urlopen(request, timeout=600) as response:
                declared = response.headers.get("Content-Length")
                if declared is not None and int(declared) > max_bytes:
                    fail(f"{url} declares {declared} bytes, over the {max_bytes} byte bound")
                temp = destination.with_suffix(destination.suffix + ".part")
                written = 0
                with temp.open("wb") as handle:
                    while True:
                        chunk = response.read(1024 * 1024)
                        if not chunk:
                            break
                        written += len(chunk)
                        if written > max_bytes:
                            handle.close()
                            temp.unlink(missing_ok=True)
                            fail(f"{url} exceeded the {max_bytes} byte bound")
                        handle.write(chunk)
                temp.rename(destination)
                return
        except (urllib.error.URLError, TimeoutError, ConnectionError) as error:
            last_error = error
        destination.unlink(missing_ok=True)
        if attempt < DOWNLOAD_ATTEMPTS:
            time.sleep(attempt * 5)
    fail(f"download {url} failed after {DOWNLOAD_ATTEMPTS} attempts: {last_error}")


def extract_archive(archive: pathlib.Path, destination: pathlib.Path, expected_root: str) -> pathlib.Path:
    destination.mkdir(parents=True, exist_ok=True)
    resolved = destination.resolve()
    with tarfile.open(archive, "r:gz") as handle:
        for member in handle.getmembers():
            target = (resolved / member.name).resolve()
            if target != resolved and resolved not in target.parents:
                fail(f"archive member escapes destination: {member.name}")
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
            elif member.issym():
                if pathlib.PurePosixPath(member.linkname).is_absolute():
                    fail(f"archive symlink is absolute: {member.name}")
                link_target = (target.parent / member.linkname).resolve()
                if link_target != resolved and resolved not in link_target.parents:
                    fail(f"archive symlink escapes destination: {member.name}")
                target.parent.mkdir(parents=True, exist_ok=True)
                if target.exists() or target.is_symlink():
                    target.unlink()
                target.symlink_to(member.linkname)
            elif member.isfile():
                source = handle.extractfile(member)
                if source is None:
                    fail(f"cannot read archive member: {member.name}")
                target.parent.mkdir(parents=True, exist_ok=True)
                with source, target.open("wb") as output:
                    shutil.copyfileobj(source, output)
            else:
                fail(f"unsupported archive member: {member.name}")
    roots = sorted(path.name for path in resolved.iterdir())
    if roots != [expected_root]:
        fail(f"{archive.name} contains {roots}, want exactly {expected_root}")
    return resolved / expected_root


def verify_checksum_inventory(root: pathlib.Path) -> Dict[str, str]:
    inventory_path = root / "SHA256SUMS"
    if not inventory_path.is_file():
        fail(f"{root.name} has no SHA256SUMS")
    inventory: Dict[str, str] = {}
    for number, line in enumerate(inventory_path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        if "  " not in line:
            fail(f"{inventory_path} line {number} is not '<sha256>  <path>'")
        digest, relative = line.split("  ", 1)
        relative = relative.removeprefix("./")
        if relative in inventory:
            fail(f"{inventory_path} lists {relative} more than once")
        inventory[relative] = digest
    for relative, expected in inventory.items():
        path = root / relative
        if not path.is_file():
            fail(f"{inventory_path} lists missing {relative}")
        actual = sha256_file(path)
        if actual != expected:
            fail(f"{relative} sha256 {actual}, inventory says {expected}")
    return inventory


def verify_package(root: pathlib.Path, target: str, identity: dict) -> Dict[str, pathlib.Path]:
    manifest_path = root / "manifest.json"
    if not manifest_path.is_file():
        fail(f"{root.name} has no manifest.json")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    release_version = identity["source_release"].removeprefix("v")
    if manifest.get("name") != "lancedb-c":
        fail(f"{root.name} manifest names {manifest.get('name')!r}")
    if manifest.get("schemaVersion") != 1:
        fail(f"{root.name} manifest schemaVersion is {manifest.get('schemaVersion')!r}")
    if manifest.get("commit") != identity["source_commit"]:
        fail(f"{root.name} manifest commit is {manifest.get('commit')!r}")
    if manifest.get("target") != target:
        fail(f"{root.name} manifest target is {manifest.get('target')!r}, want {target}")
    if manifest.get("version") != release_version:
        fail(f"{root.name} manifest version is {manifest.get('version')!r}, want {release_version}")

    expected = identity["targets"][target]
    library_path = root / expected["library"]
    if not library_path.is_file():
        fail(f"{root.name} has no library {expected['library']}")
    header_path = root / manifest["include"]
    if not header_path.is_file():
        fail(f"{root.name} has no header {manifest['include']}")
    license_path = root / manifest["license"]
    if not license_path.is_file():
        fail(f"{root.name} has no license {manifest['license']}")

    if sha256_file(library_path) != expected["library_sha256"]:
        fail(f"{target} library sha256 does not match the identity")
    if sha256_file(header_path) != identity["header_sha256"]:
        fail(f"{target} header sha256 does not match the identity")
    if sha256_file(license_path) != identity["license_sha256"]:
        fail(f"{target} license sha256 does not match the identity")

    header_text = header_path.read_text(encoding="utf-8")
    marker = f'#define LANCEDB_VERSION_STRING "{release_version}"'
    if marker not in header_text:
        fail(f"{root.name} header does not declare {marker}")

    return {"library": library_path, "license": license_path}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--identity", required=True, help="release identity JSON")
    parser.add_argument("--downloads", required=True, help="archive download directory")
    parser.add_argument("--out", required=True, help="extracted target input directory")
    parser.add_argument("--max-archive-bytes", type=int, default=MAX_ARCHIVE_BYTES_DEFAULT)
    parser.add_argument("--max-total-bytes", type=int, default=MAX_TOTAL_BYTES_DEFAULT)
    args = parser.parse_args()

    identity_path = pathlib.Path(args.identity).resolve()
    identity = json.loads(identity_path.read_text(encoding="utf-8"))
    for field in ("source_repo", "source_release", "source_commit", "header_sha256", "license_sha256", "targets"):
        if field not in identity:
            fail(f"identity is missing {field}")
    if not is_hex(identity["source_commit"], 40):
        fail("identity source_commit is not a 40-character lowercase hex commit")
    if not is_hex(identity["header_sha256"], 64) or not is_hex(identity["license_sha256"], 64):
        fail("identity header or license SHA-256 is not 64 lowercase hex characters")

    coordinates = tag_coordinates(identity["source_repo"], identity["source_release"])
    if coordinates["commit"] != identity["source_commit"]:
        fail(
            f"tag {identity['source_release']} peels to {coordinates['commit']}, "
            f"identity says {identity['source_commit']}"
        )
    for field in ("source_tag_object", "source_tag_type"):
        if field in identity and identity[field] != coordinates[field.removeprefix("source_")]:
            fail(f"tag {field} is {coordinates[field.removeprefix('source_')]}, identity says {identity[field]}")

    downloads = pathlib.Path(args.downloads).resolve()
    out = pathlib.Path(args.out).resolve()
    if out.exists() and any(out.iterdir()):
        fail(f"output directory {out} is not empty")
    out.mkdir(parents=True, exist_ok=True)
    base_url = identity.get(
        "download_base_url",
        f"https://github.com/{identity['source_repo']}/releases/download/{identity['source_release']}",
    ).rstrip("/")

    total = 0
    rows: List[str] = []
    for target in sorted(identity["targets"]):
        expected = identity["targets"][target]
        archive = expected["archive"]
        derived = f"lancedb-c-{identity['source_commit']}-{target}.tar.gz"
        if archive != derived:
            fail(f"identity archive {archive} is not the derived asset {derived}")
        archive_path = downloads / archive
        download(f"{base_url}/{archive}", archive_path, args.max_archive_bytes)
        checksum_url = f"{base_url}/{archive}.sha256"
        checksum_path = downloads / f"{archive}.sha256"
        download(checksum_url, checksum_path, 4096)
        published = checksum_path.read_text(encoding="utf-8").split()
        if len(published) < 1 or not is_hex(published[0], 64):
            fail(f"{checksum_path.name} does not start with a SHA-256")
        if published[0] != expected["archive_sha256"]:
            fail(f"{target} published checksum {published[0]} does not match the identity")
        actual = sha256_file(archive_path)
        if actual != expected["archive_sha256"]:
            fail(f"{archive} sha256 {actual}, identity says {expected['archive_sha256']}")
        total += archive_path.stat().st_size
        if total > args.max_total_bytes:
            fail(f"release archives exceed the {args.max_total_bytes} byte total bound")

        with tempfile.TemporaryDirectory(prefix=f"lancedb-c-{target}-") as temp:
            root = extract_archive(archive_path, pathlib.Path(temp), f"lancedb-c-{identity['source_commit']}-{target}")
            verify_checksum_inventory(root)
            files = verify_package(root, target, identity)
            target_dir = out / target
            target_dir.mkdir(parents=True, exist_ok=True)
            library_name = LIBRARY_NAMES.get(target)
            if library_name is None:
                fail(f"unsupported release target {target}")
            library_destination = target_dir / library_name
            shutil.copyfile(files["library"], library_destination)
            license_destination = target_dir / "LICENSE"
            shutil.copyfile(files["license"], license_destination)
        rows.append("\t".join([target, str(library_destination), str(license_destination)]))

    print("\n".join(rows))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
