#!/usr/bin/env python3
"""Loopback HTTP server for the public-verification deadline fixture.

In ``discovery`` mode every GET is accepted, logged, and held without a
response. In ``proxy`` mode the go-import discovery request is answered
immediately and every module proxy request is held. The fixture waits for the
logged request before asserting that the verifier gives up on its budget, so a
connection that is merely refused cannot satisfy the test.
"""

from __future__ import annotations

import argparse
import http.server
import pathlib
import threading
import urllib.parse

HOLD_SECONDS = 60


class Store:
    def __init__(self, mode: str, log_path: pathlib.Path):
        self.mode = mode
        self.log_path = log_path
        self.lock = threading.Lock()

    def log(self, line: str) -> None:
        with self.lock:
            with self.log_path.open("a", encoding="utf-8") as handle:
                handle.write(line + "\n")
                handle.flush()


STORE: Store


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)
        if STORE.mode == "proxy" and "go-get" in query:
            module = "go.lancedb.io" + path
            meta = f'<meta name="go-import" content="{module} mod https://go.lancedb.io/modules">'.encode()
            STORE.log(f"proxy: served discovery {path}")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(meta)))
            self.end_headers()
            self.wfile.write(meta)
            return
        STORE.log(f"{STORE.mode}: received {self.path}; holding the response")
        # Hold without writing any response. The verifier must abandon this
        # attempt at its own deadline; the fixture kills the server afterwards.
        threading.Event().wait(HOLD_SECONDS)
        self.send_response(503)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, fmt, *args):  # noqa: ANN002
        return


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["discovery", "proxy"], required=True)
    parser.add_argument("--port-file", required=True)
    parser.add_argument("--log", required=True)
    args = parser.parse_args()
    global STORE
    STORE = Store(args.mode, pathlib.Path(args.log))
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    pathlib.Path(args.port_file).write_text(str(server.server_port) + "\n", encoding="utf-8")
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
