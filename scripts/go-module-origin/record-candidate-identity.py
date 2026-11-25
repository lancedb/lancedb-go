#!/usr/bin/env python3
"""Record the candidate and manifest identity for one origin publication.

The release identity file pins the upstream lancedb-c tag, commit and archive
checksums. This command adds the generated candidate-module hashes, the module
zip/go.mod h1 values and the origin manifest SHA-256. Copy the result into
``scripts/go-module-origin/<version>.identity.json`` after a reviewed candidate
run. It never publishes anything.

Usage:
  record-candidate-identity.py --identity IN --candidates DIR --origin DIR --out OUT [--run-id ID]
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import sys


def fail(message: str) -> "None":
    raise SystemExit(f"record-candidate-identity: {message}")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def candidate_files(module_dir: pathlib.Path) -> "dict[str, pathlib.Path]":
    files = {path.name: path for path in module_dir.iterdir() if path.is_file()}
    missing = {"artifact.go", "go.mod", "LICENSE"} - files.keys()
    if missing:
        fail(f"{module_dir} is missing {sorted(missing)}")
    payloads = [name for name in files if name.endswith(".zst")]
    if len(payloads) != 1:
        fail(f"{module_dir} has {len(payloads)} payloads, want 1")
    return files


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--identity", required=True)
    parser.add_argument("--candidates", required=True)
    parser.add_argument("--origin", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--run-id", default=None)
    args = parser.parse_args()

    identity = json.loads(pathlib.Path(args.identity).read_text(encoding="utf-8"))
    targets = identity.get("targets")
    if not isinstance(targets, dict) or not targets:
        fail("identity has no targets")
    module_prefix = identity.get("module_prefix", "go.lancedb.io/lancedb-go/artifacts")

    origin = pathlib.Path(args.origin)
    manifest_path = origin / "manifest.json"
    if not manifest_path.is_file():
        fail(f"origin has no manifest: {manifest_path}")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    objects = manifest.get("objects", [])
    zip_h1 = {}
    mod_h1 = {}
    for obj in objects:
        module = obj.get("module", "")
        target = obj.get("target", "")
        if not target:
            continue
        if obj.get("key", "").endswith(".zip"):
            zip_h1[target] = obj["zip_h1"]
        if obj.get("key", "").endswith(".mod"):
            mod_h1[target] = obj["gomod_h1"]
    if set(zip_h1) != set(targets) or set(mod_h1) != set(targets):
        fail(f"manifest targets {sorted(zip_h1)} do not match identity targets {sorted(targets)}")

    candidates = pathlib.Path(args.candidates)
    for target, expected in targets.items():
        module_path = f"{module_prefix}/{target}"
        if not any(obj.get("module") == module_path for obj in objects):
            fail(f"manifest has no module {module_path}")
        files = candidate_files(candidates / target / "module")
        payload = [name for name in files if name.endswith(".zst")][0]
        expected.update(
            {
                "candidate_artifact_go_sha256": sha256_file(files["artifact.go"]),
                "candidate_go_mod_sha256": sha256_file(files["go.mod"]),
                "candidate_license_sha256": sha256_file(files["LICENSE"]),
                "candidate_payload_sha256": sha256_file(files[payload]),
                "payload_name": payload,
                "module_zip_h1": zip_h1[target],
                "module_gomod_h1": mod_h1[target],
            }
        )

    identity["manifest_sha256"] = sha256_file(manifest_path)
    if args.run_id:
        identity["candidate_run_id"] = args.run_id

    out = pathlib.Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(identity, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    print(f"recorded candidate identity: {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
