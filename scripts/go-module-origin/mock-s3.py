#!/usr/bin/env python3
"""Minimal S3 path-style mock for publisher and lock-probe tests.

It supports the subset of the S3 API the publisher uses, plus bucket-prefix
lock emulation with fault injection used to test the first-publication probe
without touching R2:

  * PUT with ``If-None-Match: *`` is create-only.
  * PUT with ``If-Match: "<etag>"`` is conditional on the exact quoted ETag.
  * With ``--lock-prefix PREFIX``, an unconditional PUT over an existing object
    under the prefix and DELETE of any object under the prefix follow the
    configured lock modes: ``deny`` returns the R2 error status/code,
    ``accept`` performs the write, ``transport`` drops the connection without a
    response, and ``transport-remove`` removes the object and then drops the
    connection.
  * ``--fail-get KEY`` drops the connection without a response for GET of one
    key (HEAD still reports existence), so a probe readback or publisher
    readback cannot silently reuse stale bytes.
"""

from __future__ import annotations

import argparse
import hashlib
import http.server
import pathlib
import urllib.parse

DEFAULT_CODE = "ObjectLockedByBucketPolicy"


def error_body(code: str) -> bytes:
    return (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        f"<Error><Code>{code}</Code>"
        "<Message>Object is protected by a bucket lock rule</Message>"
        "</Error>"
    ).encode("utf-8")


class Store:
    def __init__(self, options: argparse.Namespace):
        self.root = pathlib.Path(options.root)
        self.log = pathlib.Path(options.log)
        self.lock_prefixes = [prefix for prefix in options.lock_prefix if prefix]
        self.locked_write_mode = options.locked_write_mode
        self.locked_write_status = options.locked_write_status
        self.locked_write_code = options.locked_write_code
        self.locked_delete_mode = "accept" if options.allow_locked_delete else options.locked_delete_mode
        self.locked_delete_status = options.locked_delete_status
        self.locked_delete_code = options.locked_delete_code
        self.fail_get = set(options.fail_get or [])
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

    def drop(self):
        """Close the connection without sending any HTTP response."""
        self.close_connection = True
        try:
            self.connection.shutdown(2)
        except OSError:
            pass

    def send_locked(self, operation: str, status: int, code: str):
        STORE.write_log(f"{operation} {self.split()} {status} {code}")
        self.send_empty(status, body=error_body(code))

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
            if key in STORE.fail_get:
                STORE.write_log(f"GET {key} transport-drop")
                self.drop()
                return
            path = STORE.path(key)
            if not path.is_file():
                STORE.write_log(f"GET {key} 404")
                self.send_empty(404, body=error_body("NoSuchKey"))
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
                mode = STORE.locked_write_mode
                if mode == "transport":
                    STORE.write_log(f"PUT {key} transport-drop")
                    self.drop()
                    return
                if mode == "deny":
                    self.send_locked("PUT", STORE.locked_write_status, STORE.locked_write_code)
                    return
                # mode == accept falls through to the normal write.
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
            if STORE.locked(key):
                mode = STORE.locked_delete_mode
                if mode == "transport":
                    STORE.write_log(f"DELETE {key} transport-drop")
                    self.drop()
                    return
                if mode == "transport-remove":
                    path.unlink(missing_ok=True)
                    STORE.write_log(f"DELETE {key} transport-remove")
                    self.drop()
                    return
                if mode == "deny":
                    self.send_locked("DELETE", STORE.locked_delete_status, STORE.locked_delete_code)
                    return
                # mode == accept falls through to the normal delete.
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
    parser.add_argument("--locked-write-mode", choices=["deny", "accept", "transport"], default="deny")
    parser.add_argument("--locked-write-status", type=int, default=409)
    parser.add_argument("--locked-write-code", default=DEFAULT_CODE)
    parser.add_argument("--locked-delete-mode", choices=["deny", "accept", "transport", "transport-remove"], default="deny")
    parser.add_argument("--locked-delete-status", type=int, default=409)
    parser.add_argument("--locked-delete-code", default=DEFAULT_CODE)
    parser.add_argument("--allow-locked-delete", action="store_true", help="alias for --locked-delete-mode accept")
    parser.add_argument("--fail-get", action="append", default=[], help="repeatable key whose GET/HEAD drops the connection")
    args = parser.parse_args()
    global STORE
    STORE = Store(args)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(server.server_address[1], flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
