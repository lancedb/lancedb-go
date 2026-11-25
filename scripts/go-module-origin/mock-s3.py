#!/usr/bin/env python3
"""Minimal S3 path-style mock for publisher and lock-probe tests.

It supports the subset of the S3 API the publisher uses, plus an optional
bucket-prefix lock emulation used to test the first-publication probe without
touching R2:

  * PUT with ``If-None-Match: *`` is create-only.
  * PUT with ``If-Match: "<etag>"`` is conditional on the exact quoted ETag.
  * With ``--lock-prefix PREFIX``, an unconditional PUT over an existing object
    under the prefix and DELETE of any object under the prefix are denied with
    403 AccessDenied, like an R2 bucket lock.
  * ``--allow-locked-delete`` makes DELETE succeed under the lock prefix so a
    test can drive the probe's repair path.
"""

from __future__ import annotations

import argparse
import hashlib
import http.server
import pathlib
import sys
import urllib.parse

ACCESS_DENIED = (
    '<?xml version="1.0" encoding="UTF-8"?>\n'
    "<Error><Code>AccessDenied</Code>"
    "<Message>Object is protected by a bucket lock rule</Message>"
    "</Error>"
)


class Store:
    def __init__(self, root: pathlib.Path, log: pathlib.Path, lock_prefixes, allow_locked_delete: bool):
        self.root = root
        self.log = log
        self.lock_prefixes = [prefix for prefix in lock_prefixes if prefix]
        self.allow_locked_delete = allow_locked_delete
        self.root.mkdir(parents=True, exist_ok=True)

    def path(self, key: str) -> pathlib.Path:
        if ".." in key.split("/") or key.startswith("/"):
            raise ValueError(key)
        return self.root / key

    def etag(self, path: pathlib.Path) -> str:
        return '"' + hashlib.md5(path.read_bytes()).hexdigest() + '"'

    def locked(self, key: str) -> bool:
        return any(key.startswith(prefix) for prefix in self.lock_prefixes)

    def write_log(self, line: str) -> None:
        with self.log.open("a", encoding="utf-8") as handle:
            handle.write(line + "\n")


STORE: Store


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def split(self):
        parsed = urllib.parse.urlsplit(self.path)
        parts = parsed.path.lstrip("/").split("/", 1)
        if len(parts) != 2:
            raise ValueError(self.path)
        return urllib.parse.unquote(parts[1])

    def send_empty(self, status, headers=None, body=None):
        self.send_response(status)
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.send_header("Content-Length", str(len(body or b"")))
        if body:
            self.send_header("Content-Type", "application/xml")
        self.end_headers()
        if body:
            self.wfile.write(body)

    def send_access_denied(self, operation: str, key: str):
        STORE.write_log(f"{operation} {key} 403 locked")
        self.send_empty(403, body=ACCESS_DENIED.encode("utf-8"))

    def do_HEAD(self):
        try:
            key = self.split()
            path = STORE.path(key)
            if not path.is_file():
                STORE.write_log(f"HEAD {key} 404")
                self.send_empty(404)
                return
            STORE.write_log(f"HEAD {key} 200")
            self.send_response(200)
            self.send_header("Content-Length", str(path.stat().st_size))
            self.send_header("ETag", STORE.etag(path))
            self.send_header("Content-Type", "application/octet-stream")
            self.end_headers()
        except Exception as exc:  # noqa: BLE001
            self.send_empty(400, {"X-Error": str(exc)})

    def do_GET(self):
        try:
            key = self.split()
            path = STORE.path(key)
            if not path.is_file():
                STORE.write_log(f"GET {key} 404")
                self.send_empty(404)
                return
            body = path.read_bytes()
            STORE.write_log(f"GET {key} 200")
            self.send_response(200)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("ETag", STORE.etag(path))
            self.send_header("Content-Type", "application/octet-stream")
            self.end_headers()
            self.wfile.write(body)
        except Exception as exc:  # noqa: BLE001
            self.send_empty(400, {"X-Error": str(exc)})

    def do_PUT(self):
        try:
            key = self.split()
            path = STORE.path(key)
            length = int(self.headers.get("Content-Length", "0"))
            body = self.rfile.read(length)
            existing = path.is_file()
            create_only = self.headers.get("If-None-Match") == "*"
            if_match = self.headers.get("If-Match")
            if STORE.locked(key) and not create_only and not if_match:
                self.send_access_denied("PUT", key)
                return
            if create_only and existing:
                STORE.write_log(f"PUT {key} 412 if-none-match")
                self.send_empty(412)
                return
            # R2 requires the condition value to be the quoted ETag exactly.
            if if_match and (not existing or STORE.etag(path) != if_match):
                STORE.write_log(f"PUT {key} 412 if-match")
                self.send_empty(412)
                return
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
            STORE.write_log(
                f"PUT {key} 200 {len(body)} if-none-match={self.headers.get('If-None-Match') or '-'} if-match={if_match or '-'}"
            )
            digest = hashlib.md5(body).hexdigest()
            self.send_response(200)
            self.send_header("ETag", '"' + digest + '"')
            self.send_header("Content-Length", "0")
            self.end_headers()
        except Exception as exc:  # noqa: BLE001
            STORE.write_log(f"PUT error {exc}")
            self.send_empty(400, {"X-Error": str(exc)})

    def do_DELETE(self):
        try:
            key = self.split()
            path = STORE.path(key)
            if STORE.locked(key) and not STORE.allow_locked_delete:
                self.send_access_denied("DELETE", key)
                return
            existed = path.is_file()
            path.unlink(missing_ok=True)
            STORE.write_log(f"DELETE {key} 204 existed={existed}")
            self.send_empty(204)
        except Exception as exc:  # noqa: BLE001
            STORE.write_log(f"DELETE error {exc}")
            self.send_empty(400, {"X-Error": str(exc)})

    def log_message(self, fmt, *args):  # noqa: ANN002
        return


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--port", type=int, default=18789)
    parser.add_argument("--log", required=True)
    parser.add_argument("--lock-prefix", action="append", default=[], help="repeatable locked key prefix")
    parser.add_argument("--allow-locked-delete", action="store_true")
    args = parser.parse_args()
    global STORE
    STORE = Store(pathlib.Path(args.root), pathlib.Path(args.log), args.lock_prefix, args.allow_locked_delete)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(server.server_address[1], flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
