#!/usr/bin/env python3
# Copyright (c) 2026 Rhythm Shahriar <me@rhy.io>
# SPDX-License-Identifier: AGPL-3.0-only

"""Serves a fake GitHub API tree on 127.0.0.1 for the installer tests.

Usage: python3 -I server.py ROOT LOG [PORT]

ROOT is a tree built by mkfake.sh (or run.ps1). PORT 0, the default, picks
a free port. Once listening, the server writes its port to ROOT/.port.

Routes:
  /status/<code>/...  answers <code>; 403 and 429 carry GitHub's rate-limit headers
  /hang/...           never answers, for timeout tests
  /redirect/<host:port>/...
                      answers 302 to plain http://<host:port>/..., for the
                      redirect tests
  anything else       the file under ROOT. In files under repos/ the text
                      {{BASE}} becomes this server's URL, so the release JSON
                      points at its own downloads whatever the port.

Every request is appended to LOG as "GET <path> auth=<Authorization or ->",
so tests can assert which token, if any, reached the API.
"""

import http.server
import os
import sys
import threading
import time
from pathlib import Path


def handler(root, log, lock):
    class Handler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, directory=root, **kwargs)

        def log_message(self, format, *args):  # noqa: A002 (the base class name)
            pass  # the request log below is the only output

        def record(self):
            auth = self.headers.get("Authorization") or "-"
            with lock, open(log, "a", encoding="utf-8") as f:
                f.write(f"{self.command} {self.path} auth={auth}\n")

        def do_HEAD(self):
            self.record()
            super().do_HEAD()

        def do_GET(self):
            self.record()
            parts = self.path.split("?", 1)[0].split("/")
            if len(parts) > 2 and parts[1] == "status" and parts[2].isdigit():
                code = int(parts[2])
                self.send_response(code)
                if code in (403, 429):
                    self.send_header("X-RateLimit-Remaining", "0")
                    self.send_header("X-RateLimit-Reset", str(int(time.time()) + 3600))
                self.send_header("Content-Type", "application/json")
                body = b'{"message":"API rate limit exceeded"}' if code in (403, 429) else b"{}"
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
            if len(parts) > 2 and parts[1] == "redirect":
                rest = self.path.split("/", 3)[3] if self.path.count("/") >= 3 else ""
                self.send_response(302)
                self.send_header("Location", f"http://{parts[2]}/{rest}")
                self.send_header("Content-Length", "0")
                self.end_headers()
                return
            if len(parts) > 1 and parts[1] == "hang":
                time.sleep(3600)
                return
            if len(parts) > 1 and parts[1] == "repos":
                self.send_templated()
                return
            super().do_GET()

        def send_templated(self):
            path = Path(self.translate_path(self.path))
            if not path.is_file():
                self.send_error(404)
                return
            base = f"http://127.0.0.1:{self.server.server_address[1]}"
            body = path.read_text(encoding="utf-8").replace("{{BASE}}", base).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    return Handler


def main():
    if len(sys.argv) not in (3, 4):
        sys.exit("usage: server.py ROOT LOG [PORT]")
    root, log = os.path.abspath(sys.argv[1]), os.path.abspath(sys.argv[2])
    port = int(sys.argv[3]) if len(sys.argv) == 4 else 0
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler(root, log, threading.Lock()))
    server.daemon_threads = True
    tmp = os.path.join(root, ".port.tmp")
    with open(tmp, "w", encoding="utf-8") as f:
        f.write(str(server.server_address[1]))
    os.replace(tmp, os.path.join(root, ".port"))
    server.serve_forever()


if __name__ == "__main__":
    main()
