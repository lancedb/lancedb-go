#!/usr/bin/env python3
"""Verify candidate files against the pinned identity manifest.

Usage:
  verify-candidate-identity.py IDENTITY CANDIDATES
"""

from __future__ import annotations

import hashlib
import json
import pathlib
import sys


def fail(message: str) -> "None":
    print(f"verify-candidate-identity: {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: verify-candidate-identity.py IDENTITY CANDIDATES", file=sys.stderr)
        return 2
    identity = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
    root = pathlib.Path(sys.argv[2])
    targets = identity.get("targets")
    if not isinstance(targets, dict) or not targets:
        fail("identity has no targets")
    for target, expected in targets.items():
        module = root / target / "module"
        if not module.is_dir():
            fail(f"missing candidate module: {module}")
        payloads = list(module.glob("*.zst"))
        if len(payloads) != 1:
            fail(f"{target}: expected one payload, found {len(payloads)}")
        checks = [
            ("artifact.go", "candidate_artifact_go_sha256"),
            ("go.mod", "candidate_go_mod_sha256"),
            ("LICENSE", "candidate_license_sha256"),
            (payloads[0].name, "candidate_payload_sha256"),
        ]
        for name, field in checks:
            if field not in expected:
                fail(f"{target}: identity is missing {field}; record the candidate identity first")
            path = module / name
            if not path.is_file():
                fail(f"missing candidate file: {path}")
            got = sha256_file(path)
            if got != expected[field]:
                fail(f"{target}: {name} sha256 {got}, want {expected[field]}")
        if expected.get("payload_name") not in (None, payloads[0].name):
            fail(f"{target}: payload is {payloads[0].name}, identity says {expected['payload_name']}")
    print("candidate identity: ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
