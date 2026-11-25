#!/usr/bin/env python3
"""Minimal S3 SigV4 verifier for the signed-path fixture.

It accepts the curl/AWS CLI request form used by the lock probe, verifies the
AWS Signature Version 4 with fixed test credentials, and models R2's bucket
lock response: GET returns the staged probe object, PUT and DELETE return
HTTP 409 with ``ObjectLockedByBucketPolicy``.

The canonical URI is normalized to the URI-encoded form used by AWS CLI and R2,
so a client that signs the raw ``@v`` path (the defect this fixture covers)
fails with ``SignatureDoesNotMatch``.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import http.server
import pathlib
import re
import threading
import urllib.parse

LOCK_BODY = (
    '<?xml version="1.0" encoding="UTF-8"?>\n'
    "<Error><Code>ObjectLockedByBucketPolicy</Code>"
    "<Message>Object is protected by a bucket lock rule</Message></Error>"
).encode()
SIGNATURE_BODY = (
    '<?xml version="1.0" encoding="UTF-8"?>\n'
    "<Error><Code>SignatureDoesNotMatch</Code>"
    "<Message>The request signature we calculated does not match</Message></Error>"
).encode()
MISSING_BODY = (
    '<?xml version="1.0" encoding="UTF-8"?>\n'
    "<Error><Code>NoSuchKey</Code><Message>Not found</Message></Error>"
).encode()
EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()

AUTHORIZATION = re.compile(
    r"^AWS4-HMAC-SHA256 Credential=(?P<access>[^/]+)/(?P<date>[^/]+)/(?P<region>[^/]+)/(?P<service>[^/]+)/aws4_request, "
    r"SignedHeaders=(?P<signed>[^,]+), Signature=(?P<signature>[0-9a-f]{64})$"
)


class Store:
    def __init__(self, options: argparse.Namespace):
        self.log_path = pathlib.Path(options.log)
        self.access_key = options.access_key
        self.secret_key = options.secret_key
        self.region = options.region
        self.service = options.service
        self.probe_key = options.probe_key
        self.stage = pathlib.Path(options.stage).read_bytes()
        self.lock = threading.Lock()

    def log(self, line: str) -> None:
        with self.lock:
            with self.log_path.open("a", encoding="utf-8") as handle:
                handle.write(line + "\n")
                handle.flush()


STORE: Store


def canonical_uri(path: str) -> str:
    # R2/AWS canonicalize the URI-encoded path. Normalizing through unquote and
    # quote makes '%40' and a raw '@' map to the same canonical '%40' form.
    return urllib.parse.quote(urllib.parse.unquote(path), safe="/")


def canonical_headers(header_names: list[str], handler: http.server.BaseHTTPRequestHandler) -> str:
    lines = []
    for name in header_names:
        value = handler.headers.get(name)
        if value is None:
            return ""
        value = re.sub(r"\s+", " ", value.strip())
        lines.append(f"{name}:{value}")
    return "\n".join(lines) + "\n"


def signing_key(secret: str, date: str, region: str, service: str) -> bytes:
    key = ("AWS4" + secret).encode()
    for message in (date, region, service, "aws4_request"):
        key = hmac.new(key, message.encode(), hashlib.sha256).digest()
    return key


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def respond(self, status: int, body: bytes = b"", content_type: str = "application/xml") -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def do_GET(self):
        self.serve_request()

    def do_PUT(self):
        self.serve_request()

    def do_DELETE(self):
        self.serve_request()

    def serve_request(self):
        parsed = urllib.parse.urlsplit(self.path)
        path = parsed.path
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b""

        authorization = self.headers.get("Authorization", "")
        match = AUTHORIZATION.match(authorization)
        if not match:
            STORE.log(f"{self.command} {path} signature=missing")
            self.respond(403, SIGNATURE_BODY)
            return
        if match.group("access") != STORE.access_key:
            STORE.log(f"{self.command} {path} signature=wrong-access-key")
            self.respond(403, SIGNATURE_BODY)
            return
        if match.group("region") != STORE.region or match.group("service") != STORE.service:
            STORE.log(f"{self.command} {path} signature=wrong-scope")
            self.respond(403, SIGNATURE_BODY)
            return

        signed_headers = match.group("signed").split(";")
        amz_date = self.headers.get("X-Amz-Date", "")
        if not amz_date or amz_date[:8] != match.group("date"):
            STORE.log(f"{self.command} {path} signature=wrong-date")
            self.respond(403, SIGNATURE_BODY)
            return
        payload_hash = self.headers.get("X-Amz-Content-Sha256", "")
        if payload_hash != hashlib.sha256(body).hexdigest():
            STORE.log(f"{self.command} {path} signature=wrong-payload-hash")
            self.respond(403, SIGNATURE_BODY)
            return

        canonical_request = "\n".join(
            [
                self.command,
                canonical_uri(path),
                "",
                canonical_headers(signed_headers, self).rstrip("\n") + "\n",
                ";".join(signed_headers),
                payload_hash,
            ]
        )
        scope = f"{match.group('date')}/{STORE.region}/{STORE.service}/aws4_request"
        string_to_sign = "\n".join(
            [
                "AWS4-HMAC-SHA256",
                amz_date,
                scope,
                hashlib.sha256(canonical_request.encode()).hexdigest(),
            ]
        )
        expected = hmac.new(
            signing_key(STORE.secret_key, match.group("date"), STORE.region, STORE.service),
            string_to_sign.encode(),
            hashlib.sha256,
        ).hexdigest()
        if not hmac.compare_digest(expected, match.group("signature")):
            STORE.log(f"{self.command} {path} signature=invalid canonical_path={canonical_uri(path)}")
            self.respond(403, SIGNATURE_BODY)
            return

        decoded_path = urllib.parse.unquote(path)
        if not decoded_path.endswith("/" + STORE.probe_key):
            STORE.log(f"{self.command} {path} signature=valid path=unexpected")
            self.respond(404, MISSING_BODY)
            return

        STORE.log(
            f"{self.command} {path} signature=valid canonical_path={canonical_uri(path)}"
        )
        if self.command == "GET":
            self.respond(200, STORE.stage, "application/json")
            return
        if self.command in ("PUT", "DELETE"):
            self.respond(409, LOCK_BODY)
            return
        self.respond(405)

    def log_message(self, fmt, *args):  # noqa: ANN002
        return


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port-file", required=True)
    parser.add_argument("--log", required=True)
    parser.add_argument("--access-key", required=True)
    parser.add_argument("--secret-key", required=True)
    parser.add_argument("--region", default="auto")
    parser.add_argument("--service", default="s3")
    parser.add_argument("--probe-key", required=True)
    parser.add_argument("--stage", required=True)
    args = parser.parse_args()
    global STORE
    STORE = Store(args)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    pathlib.Path(args.port_file).write_text(str(server.server_port) + "\n", encoding="utf-8")
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
