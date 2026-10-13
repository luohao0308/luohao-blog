#!/usr/bin/env python3
"""Read-only smoke checks for the strictly allowlisted public Kuma status page."""
import argparse
import json
import re
import urllib.error
import urllib.request
from urllib.parse import urljoin, urlsplit


def check(ok, label):
    if not ok:
        raise AssertionError(label)
    print("PASS", label)


def request(base, path, method="GET", follow_redirects=True):
    handlers = []
    if not follow_redirects:
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, headers, newurl):
                return None

        handlers.append(NoRedirect())
    opener = urllib.request.build_opener(*handlers)
    req = urllib.request.Request(
        base + path,
        method=method,
        data=b"{}" if method == "POST" else None,
        headers={"Content-Type": "application/json"},
    )
    try:
        with opener.open(req, timeout=20) as response:
            return response.status, response.headers, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.headers, b""


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="https://status.makerhao.cn")
    parser.add_argument(
        "--allow-http",
        action="store_true",
        help="permit an HTTP base only for an isolated local Caddy/stub regression",
    )
    args = parser.parse_args()
    base = args.base.rstrip("/")
    parts = urlsplit(base)
    check(parts.scheme == "https" or (args.allow_http and parts.scheme == "http"),
          "HTTPS target (or explicitly allowed isolated HTTP target)")

    status, headers, _ = request(base, "/", follow_redirects=False)
    check(status == 308, "GET / redirects with 308")
    check(urljoin(base + "/", headers.get("Location", "")) == base + "/status/blog",
          "root redirect targets /status/blog")

    status, _, html_bytes = request(base, "/status/blog")
    check(status == 200, "public status page")
    html = html_bytes.decode("utf-8", errors="replace")

    for path, label in (
        ("/api/status-page/blog", "public status API"),
        ("/api/status-page/heartbeat/blog", "public heartbeat API"),
        ("/api/status-page/blog/manifest.json", "status manifest API"),
    ):
        status, headers, body = request(base, path)
        check(status == 200, label)
        value = json.loads(body)
        check(isinstance(value, dict), label + " returns a JSON object")
        expected = {
            "/api/status-page/blog": ("config", "publicGroupList"),
            "/api/status-page/heartbeat/blog": ("heartbeatList", "uptimeList"),
            "/api/status-page/blog/manifest.json": ("icons", "start_url"),
        }[path]
        check(all(key in value for key in expected), label + " has expected fields")
        check("application/json" in headers.get("Content-Type", ""), label + " has JSON content type")

    assets = sorted(set(re.findall(r'(?:src|href)=["\'](/assets/[^"\']+)', html)))
    check(bool(assets), "status page references allowlisted assets")
    for asset in assets:
        status, headers, _ = request(base, asset)
        check(status == 200 and "text/html" not in headers.get("Content-Type", ""),
              "status asset " + asset.split("?", 1)[0])

    for path in ("/status/blog", "/api/status-page/blog", "/api/status-page/heartbeat/blog", assets[0]):
        status, _, _ = request(base, path, method="HEAD")
        check(status == 200, "allowed HEAD " + path)
    status, headers, _ = request(base, "/", method="HEAD", follow_redirects=False)
    check(status == 308 and urljoin(base + "/", headers.get("Location", "")) == base + "/status/blog",
          "HEAD / redirects to public status page")

    for path in (
        "/dashboard",
        "/login",
        "/metrics",
        "/socket.io/",
        "/api/unknown",
        "/api/status-page/private",
        "/api/status-page/blog/unknown",
        "/api/status-page/heartbeat/private",
        "/assets/../dashboard",
        "/upload/example.png",
        "/screenshots/example.png",
        "/status/private",
    ):
        status, _, _ = request(base, path)
        check(status == 404, "blocked GET " + path)

    for path in ("/", "/status/blog", "/api/status-page/heartbeat/blog", "/assets/example.js"):
        status, _, _ = request(base, path, method="POST", follow_redirects=False)
        check(status == 404, "blocked POST " + path)


if __name__ == "__main__":
    main()
