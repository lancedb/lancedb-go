#!/usr/bin/env python3
"""Serve a generated origin tree for local discovery and proxy tests."""

from __future__ import annotations

import argparse
import http.server
import os
import sys


class Handler(http.server.SimpleHTTPRequestHandler):
    def guess_type(self, path):  # noqa: ANN001
        if "/modules/" not in path and "/lancedb-go/artifacts/" in path:
            return "text/html; charset=utf-8"
        if path.endswith(".info") or path.endswith(".json"):
            return "application/json"
        if path.endswith(".zip"):
            return "application/zip"
        if path.endswith(".mod") or path.endswith("/list"):
            return "text/plain; charset=utf-8"
        return super().guess_type(path)

    def log_message(self, fmt, *args):  # noqa: ANN002
        # Keep test output quiet; failures are visible in the client.
        return


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--port", type=int, default=18787)
    args = parser.parse_args()
    root = os.path.abspath(args.root)
    if not os.path.isdir(root):
        print(f"not a directory: {root}", file=sys.stderr)
        return 2
    os.chdir(root)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(server.server_address[1], flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
