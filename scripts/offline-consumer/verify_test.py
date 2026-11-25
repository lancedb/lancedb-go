#!/usr/bin/env python3
"""Synthetic boundary tests for the offline-consumer evidence helper.

These tests exercise verify.py with local fixtures. They do not download Go
modules, build binaries or load a native library, so a passing run only shows
that the evidence checks fail for the right reasons.
"""

import contextlib
import hashlib
import io
import json
import os
import stat
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify  # noqa: E402

MODULE = "go.lancedb.io/lancedb-go/artifacts/linux-amd64"
VERSION = "v0.1.0-preview.4"
PAYLOAD = b"\x28\xb5\x2f\xfd synthetic payload bytes"
ARTIFACT_GO = '''// Code generated; DO NOT EDIT.

// Package artifact embeds one compressed lancedb-c shared library as data.
package artifact

import _ "embed"

//go:embed libfake.so.zst
var Payload []byte

const (
	Target              = "linux/amd64"
	LibraryName         = "libfake.so"
	CompressedSize    int64 = %d
	CompressedSHA256    = "%s"
	UncompressedSize  int64 = 7
	UncompressedSHA256  = "%s"
	UpstreamRepo        = "lancedb/lancedb-c"
	UpstreamCommit      = "%s"
	HeaderSHA256        = "%s"
)
''' % (
    len(PAYLOAD),
    hashlib.sha256(PAYLOAD).hexdigest(),
    hashlib.sha256(b"library").hexdigest(),
    "a" * 40,
    "b" * 64,
)


def write_artifact(tmp, *, artifact_go=ARTIFACT_GO, go_mod=None):
    zip_path = Path(tmp) / "artifact.zip"
    if go_mod is None:
        go_mod = "module %s\n\ngo 1.21.0\n" % MODULE
    prefix = "%s@%s/" % (MODULE, VERSION)
    with zipfile.ZipFile(zip_path, "w") as archive:
        archive.writestr(prefix + "artifact.go", artifact_go)
        archive.writestr(prefix + "go.mod", go_mod)
        archive.writestr(prefix + "libfake.so.zst", PAYLOAD)
    mod_path = Path(tmp) / "artifact.mod"
    mod_path.write_text(go_mod, encoding="utf-8")
    return zip_path, mod_path


def run_artifact(tmp, zip_path, mod_path):
    out = Path(tmp) / "artifact.json"
    args = [
        "artifact",
        "--zip", str(zip_path),
        "--mod", str(mod_path),
        "--module", MODULE,
        "--expected-target", "linux/amd64",
        "--expected-library", "libfake.so",
        "--expected-upstream-commit", "a" * 40,
        "--out", str(out),
    ]
    return verify.main(args), out


def run_main(arguments):
    stdout = io.StringIO()
    stderr = io.StringIO()
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        code = verify.main(arguments)
    return code, stdout.getvalue(), stderr.getvalue()


class ArtifactTests(unittest.TestCase):
    def test_accepts_consistent_archive(self):
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, mod_path = write_artifact(tmp)
            code, out = run_artifact(tmp, zip_path, mod_path)
            self.assertEqual(code, 0)
            report = json.loads(out.read_text(encoding="utf-8"))
            self.assertEqual(report["module"], MODULE)
            self.assertEqual(report["uncompressed_size"], 7)

    def test_rejects_wrong_target(self):
        with tempfile.TemporaryDirectory() as tmp:
            artifact_go = ARTIFACT_GO.replace('"linux/amd64"', '"linux/arm64"', 1)
            zip_path, mod_path = write_artifact(tmp, artifact_go=artifact_go)
            code, _ = run_artifact(tmp, zip_path, mod_path)
            self.assertNotEqual(code, 0)

    def test_rejects_payload_digest_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            artifact_go = ARTIFACT_GO.replace(
                hashlib.sha256(PAYLOAD).hexdigest(), "0" * 64, 1)
            zip_path, mod_path = write_artifact(tmp, artifact_go=artifact_go)
            code, _ = run_artifact(tmp, zip_path, mod_path)
            self.assertNotEqual(code, 0)

    def test_rejects_newer_go_directive(self):
        with tempfile.TemporaryDirectory() as tmp:
            zip_path, mod_path = write_artifact(
                tmp, go_mod="module %s\n\ngo 1.26.0\n" % MODULE)
            code, _ = run_artifact(tmp, zip_path, mod_path)
            self.assertNotEqual(code, 0)

    def test_rejects_artifact_requirements(self):
        with tempfile.TemporaryDirectory() as tmp:
            go_mod = "module %s\n\ngo 1.21.0\n\nrequire example.com/x v1.0.0\n" % MODULE
            zip_path, mod_path = write_artifact(tmp, go_mod=go_mod)
            code, _ = run_artifact(tmp, zip_path, mod_path)
            self.assertNotEqual(code, 0)


class BinaryTests(unittest.TestCase):
    def metadata(self, *, cgo="0", deps=(), goos="linux", goarch="amd64"):
        lines = [
            "fixture: go1.21.13",
            "\tpath\texample.com/offline-consumer",
            "\tmod\texample.com/offline-consumer\t(devel)\t",
        ]
        for dep in deps:
            lines.append(dep)
        lines.extend([
            "\tbuild\t-buildmode=exe",
            "\tbuild\tCGO_ENABLED=%s" % cgo,
            "\tbuild\tGOOS=%s" % goos,
            "\tbuild\tGOARCH=%s" % goarch,
        ])
        return "\n".join(lines) + "\n"

    def run_binary(self, tmp, metadata):
        path = Path(tmp) / "metadata.txt"
        path.write_text(metadata, encoding="utf-8")
        return run_main([
            "binary",
            "--metadata", str(path),
            "--root-module", "github.com/lancedb/lancedb-go",
            "--root-version", "v0.2.0-preview.3",
            "--artifact-prefix", "go.lancedb.io/lancedb-go/artifacts",
            "--slug", "linux-amd64",
            "--artifact-version", VERSION,
            "--goos", "linux",
            "--goarch", "amd64",
        ])

    def test_accepts_matching_binary(self):
        deps = (
            "\tdep\tgithub.com/lancedb/lancedb-go\tv0.2.0-preview.3\th1:root",
            "\tdep\t%s\t%s\th1:artifact" % (MODULE, VERSION),
        )
        with tempfile.TemporaryDirectory() as tmp:
            code, _, _ = self.run_binary(tmp, self.metadata(deps=deps))
            self.assertEqual(code, 0)

    def test_rejects_cgo_binary(self):
        deps = (
            "\tdep\tgithub.com/lancedb/lancedb-go\tv0.2.0-preview.3\th1:root",
            "\tdep\t%s\t%s\th1:artifact" % (MODULE, VERSION),
        )
        with tempfile.TemporaryDirectory() as tmp:
            code, _, _ = self.run_binary(tmp, self.metadata(cgo="1", deps=deps))
            self.assertNotEqual(code, 0)

    def test_rejects_two_artifact_dependencies(self):
        deps = (
            "\tdep\tgithub.com/lancedb/lancedb-go\tv0.2.0-preview.3\th1:root",
            "\tdep\t%s\t%s\th1:artifact" % (MODULE, VERSION),
            "\tdep\tgo.lancedb.io/lancedb-go/artifacts/linux-arm64\t%s\th1:other" % VERSION,
        )
        with tempfile.TemporaryDirectory() as tmp:
            code, _, _ = self.run_binary(tmp, self.metadata(deps=deps))
            self.assertNotEqual(code, 0)


class CacheTests(unittest.TestCase):
    def build_cache(self, tmp, *, mode=0o500, extra=False, content=b"library"):
        root = Path(tmp) / "cache"
        entry = root / "lancedb-go" / "native" / "0.1.0-preview.4" / "linux-amd64"
        entry.mkdir(parents=True)
        (entry / ".lock").write_bytes(b"")
        library = entry / "libfake.so"
        library.write_bytes(content)
        if os.name != "nt":
            os.chmod(library, mode)
        if extra:
            (entry / "libfake.so.bak").write_bytes(b"extra")
        return root, hashlib.sha256(content).hexdigest()

    def run_cache(self, root, sha, size=7):
        return run_main([
            "cache",
            "--cache-root", str(root),
            "--release-version", "0.1.0-preview.4",
            "--slug", "linux-amd64",
            "--library", "libfake.so",
            "--size", str(size),
            "--sha", sha,
        ])

    def test_accepts_expected_entry(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, sha = self.build_cache(tmp)
            code, _, _ = self.run_cache(root, sha)
            self.assertEqual(code, 0)

    def test_rejects_unexpected_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, sha = self.build_cache(tmp, extra=True)
            code, _, _ = self.run_cache(root, sha)
            self.assertNotEqual(code, 0)

    def test_rejects_digest_mismatch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, _ = self.build_cache(tmp)
            code, _, _ = self.run_cache(root, "0" * 64)
            self.assertNotEqual(code, 0)

    @unittest.skipIf(os.name == "nt", "POSIX permission check")
    def test_rejects_unsafe_mode(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, sha = self.build_cache(tmp, mode=0o600)
            code, _, _ = self.run_cache(root, sha)
            self.assertNotEqual(code, 0)


if __name__ == "__main__":
    unittest.main()
