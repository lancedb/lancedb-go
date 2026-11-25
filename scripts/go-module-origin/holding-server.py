#!/usr/bin/env python3
"""Loopback HTTP server for the public-verification deadline fixture.

In ``discovery`` mode every GET is accepted, logged, and held without a
response. In ``proxy`` mode the go-import discovery request is answered after
an optional delay and every module proxy request is held. Requests and closes
are logged with monotonic timestamps so the fixture can measure the remaining
budget after a preceding successful stage. The fixture waits for the logged
request before asserting that the verifier gives up on its budget, so a
connection that is merely refused cannot satisfy the test.
"""

from __future__ import annotations

import argparse
import http.server
import pathlib
import socket
import threading
import time
import urllib.parse

HOLD_SECONDS = 60


class Store:
    def __init__(self, mode: str, log_path: pathlib.Path, discovery_delay: float):
        self.mode = mode
        self.log_path = log_path
        self.discovery_delay = discovery_delay
        self.lock = threading.Lock()

    def log(self, line: str) -> None:
        with self.lock:
            with self.log_path.open("a", encoding="utf-8") as handle:
                handle.write(f"{time.monotonic():.6f} {line}\n")
                handle.flush()


STORE: Store


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def hold(self, path: str) -> None:
        STORE.log(f"{STORE.mode}: received {path}; holding")
        self.connection.settimeout(HOLD_SECONDS)
        try:
            while self.connection.recv(4096):
                pass
        except (socket.timeout, ConnectionResetError, OSError):
            pass
        STORE.log(f"{STORE.mode}: closed {path}")
        self.close_connection = True

    def do_GET(self):
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)
        if STORE.mode in ("proxy", "missing") and "go-get" in query:
            STORE.log(f"{STORE.mode}: received discovery {path}")
            if STORE.discovery_delay > 0:
                time.sleep(STORE.discovery_delay)
            module = "go.lancedb.io" + path
            meta = f'<meta name="go-import" content="{module} mod https://go.lancedb.io/modules">'.encode()
            STORE.log(f"{STORE.mode}: served discovery {path}")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(meta)))
            self.end_headers()
            self.wfile.write(meta)
            return
        if STORE.mode == "missing":
            STORE.log(f"missing: {path} 404")
            self.send_response(404)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self.hold(self.path)

    def log_message(self, fmt, *args):  # noqa: ANN002
        return


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["discovery", "proxy", "missing"], required=True)
    parser.add_argument("--port-file", required=True)
    parser.add_argument("--log", required=True)
    parser.add_argument("--discovery-delay", type=float, default=0.0)
    args = parser.parse_args()
    global STORE
    STORE = Store(args.mode, pathlib.Path(args.log), args.discovery_delay)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    pathlib.Path(args.port_file).write_text(str(server.server_port) + "\n", encoding="utf-8")
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
