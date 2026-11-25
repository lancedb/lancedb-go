#!/usr/bin/env python3
"""Independent evidence checks for the offline-consumer lane.

The shell driver records downloads and runs the consumers. This helper only
inspects the downloaded module archive, the built binary metadata and the
extracted native cache. It deliberately does not invoke Go, build anything or
fall back to another candidate.
"""

import argparse
import hashlib
import json
import os
import re
import stat
import sys
import zipfile
from pathlib import Path

CONSTANT_NAMES = {
    "Target",
    "LibraryName",
    "CompressedSize",
    "CompressedSHA256",
    "UncompressedSize",
    "UncompressedSHA256",
    "SourceRepo",
    "SourceRelease",
    "SourceCommit",
    "HeaderSHA256",
}
HEX40 = re.compile(r"^[0-9a-f]{40}$")
HEX64 = re.compile(r"^[0-9a-f]{64}$")


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse_constants(source):
    constants = {}
    for line in source.splitlines():
        line = line.split("//", 1)[0].strip()
        if "=" not in line:
            continue
        lhs, rhs = line.split("=", 1)
        parts = lhs.split()
        if parts and parts[-1] == "int64":
            parts = parts[:-1]
        if len(parts) != 1 or parts[0] not in CONSTANT_NAMES:
            continue
        rhs = rhs.strip().rstrip(",")
        if rhs.startswith('"') and rhs.endswith('"'):
            constants[parts[0]] = rhs[1:-1]
        else:
            try:
                constants[parts[0]] = int(rhs)
            except ValueError:
                continue
    return constants


def go_directive_at_most(value, ceiling):
    def parts(text):
        fields = (text.split(".") + ["0", "0"])[:3]
        return tuple(int(field) for field in fields)

    return parts(value) <= parts(ceiling)


def cmd_artifact(args):
    errors = []
    artifact_source = None
    payload = None
    payload_name = None
    prefix = None
    with zipfile.ZipFile(args.zip) as archive:
        names = archive.namelist()
        for name in names:
            if name.endswith("/go.mod"):
                prefix = name[: -len("go.mod")]
            if name.endswith("/artifact.go"):
                artifact_source = archive.read(name).decode("utf-8")
            if name.endswith(".zst"):
                payload_name = name
                payload = archive.read(name)
        if prefix is None or artifact_source is None or payload is None:
            print("artifact: archive does not contain go.mod, artifact.go and a .zst payload", file=sys.stderr)
            return 1
        if not prefix.startswith(args.module + "@"):
            errors.append("archive members do not start with %s@: %s" % (args.module, prefix))
        if "package artifact" not in artifact_source:
            errors.append("artifact.go does not declare package artifact")

    constants = parse_constants(artifact_source)
    missing = sorted(CONSTANT_NAMES - set(constants))
    if missing:
        errors.append("artifact.go is missing constants: %s" % ", ".join(missing))
    for field, pattern in (("CompressedSHA256", HEX64), ("UncompressedSHA256", HEX64),
                           ("SourceCommit", HEX40), ("HeaderSHA256", HEX64)):
        if field in constants and not pattern.match(str(constants[field])):
            errors.append("%s is not the expected lowercase hex form: %r" % (field, constants[field]))
    if constants.get("Target") != args.expected_target:
        errors.append("Target is %r, want %r" % (constants.get("Target"), args.expected_target))
    if constants.get("LibraryName") != args.expected_library:
        errors.append("LibraryName is %r, want %r" % (constants.get("LibraryName"), args.expected_library))
    if constants.get("SourceRelease") != args.expected_source_release:
        errors.append("SourceRelease is %r, want %r" % (constants.get("SourceRelease"), args.expected_source_release))
    if constants.get("CompressedSize") != len(payload):
        errors.append("CompressedSize is %r, payload has %d bytes" % (constants.get("CompressedSize"), len(payload)))
    if constants.get("CompressedSHA256") != hashlib.sha256(payload).hexdigest():
        errors.append("compressed payload sha256 does not match CompressedSHA256")
    if not isinstance(constants.get("UncompressedSize"), int) or constants["UncompressedSize"] <= 0:
        errors.append("UncompressedSize is not a positive integer: %r" % constants.get("UncompressedSize"))

    mod_text = Path(args.mod).read_text(encoding="utf-8")
    if not re.search(r"(?m)^module %s$" % re.escape(args.module), mod_text):
        errors.append("artifact go.mod does not declare module %s" % args.module)
    go_match = re.search(r"(?m)^go ([\d.]+)$", mod_text)
    if not go_match:
        errors.append("artifact go.mod has no go directive")
    elif not go_directive_at_most(go_match.group(1), args.max_go):
        errors.append("artifact go.mod requires go %s, ceiling is %s" % (go_match.group(1), args.max_go))
    if re.search(r"(?m)^\s*require\b", mod_text):
        errors.append("artifact go.mod declares requirements")

    if errors:
        for error in errors:
            print("artifact: %s" % error, file=sys.stderr)
        return 1

    report = {
        "module": args.module,
        "target": constants["Target"],
        "library_name": constants["LibraryName"],
        "compressed_size": constants["CompressedSize"],
        "compressed_sha256": constants["CompressedSHA256"],
        "uncompressed_size": constants["UncompressedSize"],
        "uncompressed_sha256": constants["UncompressedSHA256"],
        "source_repo": constants["SourceRepo"],
        "source_release": constants["SourceRelease"],
        "source_commit": constants["SourceCommit"],
        "header_sha256": constants["HeaderSHA256"],
        "payload_member": payload_name,
    }
    Path(args.out).write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print("artifact %s: %s %d -> %d bytes" % (
        args.module, constants["LibraryName"], constants["UncompressedSize"], constants["CompressedSize"]))
    return 0


def cmd_binary(args):
    errors = []
    metadata = Path(args.metadata).read_text(encoding="utf-8")
    build = {}
    deps = []
    for line in metadata.splitlines():
        fields = line.strip().split("\t")
        if len(fields) == 2 and fields[0] == "build" and "=" in fields[1]:
            key, value = fields[1].split("=", 1)
            build[key] = value
        elif len(fields) >= 3 and fields[0] == "dep":
            deps.append(fields[1:])

    if build.get("CGO_ENABLED") != "0":
        errors.append("binary CGO_ENABLED is %r, want 0" % build.get("CGO_ENABLED"))
    if build.get("GOOS") != args.goos:
        errors.append("binary GOOS is %r, want %r" % (build.get("GOOS"), args.goos))
    if build.get("GOARCH") != args.goarch:
        errors.append("binary GOARCH is %r, want %r" % (build.get("GOARCH"), args.goarch))
    root = [dep for dep in deps if dep[0] == args.root_module]
    if len(root) != 1:
        errors.append("binary records %d %s dependencies, want exactly 1" % (len(root), args.root_module))
    elif root[0][1] != args.root_version:
        errors.append("binary builds %s %s, want %s" % (args.root_module, root[0][1], args.root_version))
    selected = [dep for dep in deps if dep[0].startswith(args.artifact_prefix + "/")]
    if len(selected) != 1:
        errors.append("binary records %d artifact dependencies, want exactly 1: %s" % (
            len(selected), ", ".join(dep[0] for dep in selected)))
    else:
        want = "%s/%s" % (args.artifact_prefix, args.slug)
        if selected[0][0] != want:
            errors.append("binary artifact dependency is %s, want %s" % (selected[0][0], want))
        if selected[0][1] != args.artifact_version:
            errors.append("binary artifact version is %s, want %s" % (selected[0][1], args.artifact_version))

    if errors:
        for error in errors:
            print("binary: %s" % error, file=sys.stderr)
        return 1
    print("binary: %s %s, %s %s, CGO_ENABLED=0" % (
        args.root_module, args.root_version, selected[0][0], selected[0][1]))
    return 0


def cmd_cache(args):
    errors = []
    root = Path(args.cache_root)
    if not root.is_dir():
        print("cache: %s is not a directory" % root, file=sys.stderr)
        return 1
    expected = root / "lancedb-go" / "native" / args.release_version / args.slug / args.library
    found = []
    for path in root.rglob("*"):
        if path.is_file() and path.name != ".lock":
            found.append(path)
    if found != [expected]:
        errors.append("cache files are %s, want exactly [%s]" % (
            ", ".join(str(path) for path in found), expected))
    for path in root.rglob("*"):
        if ".tmp-" in path.name:
            errors.append("cache contains a temporary entry: %s" % path)
    if expected.is_file():
        if expected.stat().st_size != args.size:
            errors.append("cached library size is %d, want %d" % (expected.stat().st_size, args.size))
        if sha256_file(expected) != args.sha:
            errors.append("cached library sha256 does not match the artifact payload")
        if os.name != "nt":
            mode = stat.S_IMODE(expected.stat().st_mode)
            if mode != 0o500:
                errors.append("cached library mode is %o, want 500" % mode)

    if errors:
        for error in errors:
            print("cache: %s" % error, file=sys.stderr)
        return 1
    print("cache: %s matches the artifact payload" % expected)
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    artifact = subparsers.add_parser("artifact", help="verify one downloaded artifact module archive")
    artifact.add_argument("--zip", required=True)
    artifact.add_argument("--mod", required=True)
    artifact.add_argument("--module", required=True)
    artifact.add_argument("--expected-target", required=True)
    artifact.add_argument("--expected-library", required=True)
    artifact.add_argument("--expected-source-release", required=True)
    artifact.add_argument("--max-go", default="1.21.0")
    artifact.add_argument("--out", required=True)
    artifact.set_defaults(func=cmd_artifact)

    binary = subparsers.add_parser("binary", help="verify built binary module metadata")
    binary.add_argument("--metadata", required=True)
    binary.add_argument("--root-module", required=True)
    binary.add_argument("--root-version", required=True)
    binary.add_argument("--artifact-prefix", required=True)
    binary.add_argument("--slug", required=True)
    binary.add_argument("--artifact-version", required=True)
    binary.add_argument("--goos", required=True)
    binary.add_argument("--goarch", required=True)
    binary.set_defaults(func=cmd_binary)

    cache = subparsers.add_parser("cache", help="verify the extracted native cache entry")
    cache.add_argument("--cache-root", required=True)
    cache.add_argument("--release-version", required=True)
    cache.add_argument("--slug", required=True)
    cache.add_argument("--library", required=True)
    cache.add_argument("--size", required=True, type=int)
    cache.add_argument("--sha", required=True)
    cache.set_defaults(func=cmd_cache)

    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
