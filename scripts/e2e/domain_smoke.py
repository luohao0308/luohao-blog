#!/usr/bin/env python3
"""Read-only domain smoke checks; anonymous logout only expires an empty session."""
import argparse
import json
import re
import urllib.request
from http.cookies import SimpleCookie
from urllib.parse import urlsplit


def check(ok, label):
    if not ok:
        raise AssertionError(label)
    print("PASS", label)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="https://makerhao.cn")
    parser.add_argument("--icp", default="渝ICP备2026025040号-1")
    args = parser.parse_args()
    base = args.base.rstrip("/")
    check(urlsplit(base).scheme == "https", "HTTPS target")

    def fetch(path, method="GET"):
        req = urllib.request.Request(base + path, method=method,
                                     data=b"{}" if method == "POST" else None,
                                     headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(req, timeout=20) as response:
            check(response.status == 200, method + " " + path.split("?")[0])
            return response.headers, response.read().decode()

    _, html = fetch("/?domain_probe=1")
    print("PASS HTTPS connection with system CA verification")
    check(args.icp in html and "https://beian.miit.gov.cn/" in html, "ICP footer and official link")
    check(bool(re.search(r'<link[^>]*rel="canonical"[^>]*href="' + re.escape(base + "/") + '"', html)),
          "canonical origin excludes query parameters")
    check("http://193.112.128.245" not in html, "no legacy HTTP IP in SSR page")
    _, body = fetch("/api/v1/articles/list?page_size=1")
    check(isinstance(json.loads(body), dict), "BFF public API JSON")
    _, html = fetch("/login")
    check(bool(re.search(r'<meta[^>]*name="robots"[^>]*content="noindex', html)), "login noindex")
    headers, _ = fetch("/api/v1/auth/logout", "POST")
    cookies = SimpleCookie()
    for raw in headers.get_all("Set-Cookie", []):
        cookies.load(raw)
    cookie = cookies.get("refresh_token")
    check(cookie is not None, "refresh cookie expiry returned")
    check(bool(cookie["secure"]) and bool(cookie["httponly"]), "Secure and HttpOnly cookie")
    check(cookie["path"] == "/api/v1/auth", "BFF cookie Path rewrite")
    check(cookie["samesite"].lower() == "lax" and cookie["max-age"] == "-1", "logout expires SameSite cookie")


if __name__ == "__main__":
    main()
