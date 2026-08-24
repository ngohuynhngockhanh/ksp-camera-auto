#!/usr/bin/env python3
"""Authenticated RedBida acceptance probe; never prints protected values."""

import argparse
import json
import sys
import urllib.parse
import urllib.request
from http.cookiejar import CookieJar


def request_json(opener, url, method="GET", payload=None):
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    with opener.open(request, timeout=45) as response:
        return json.load(response)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:2028")
    parser.add_argument("--user", required=True)
    parser.add_argument("--password-stdin", action="store_true")
    parser.add_argument("--apply-key")
    parser.add_argument("--apply-value")
    args = parser.parse_args()

    if not args.password_stdin:
        parser.error("--password-stdin is required so credentials do not appear in process arguments")
    password = sys.stdin.readline().rstrip("\r\n")
    if not password:
        parser.error("password stdin is empty")

    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(CookieJar()))
    login = urllib.parse.urlencode({"username": args.user, "password": password}).encode("utf-8")
    with opener.open(urllib.request.Request(args.base + "/login", data=login, method="POST"), timeout=15) as response:
        if response.status != 200:
            raise RuntimeError("login failed")

    catalog = request_json(opener, args.base + "/api/redbida/catalog")
    keys = [item["key"] for item in catalog.get("keys", [])]
    refresh = request_json(opener, args.base + "/api/redbida/refresh", "POST", {"keys": keys})
    values = refresh.get("values", [])
    by_key = {item["key"]: item for item in values}
    summary = {
        "catalogKeys": len(keys),
        "sourceAvailable": catalog.get("sourceAvailable"),
        "existingKeys": sum(1 for item in values if item.get("exists")),
        "missingKeys": sum(1 for item in values if not item.get("exists")),
        "protectedMasked": sum(1 for item in values if item.get("meta", {}).get("secret") and item.get("value") == "********"),
        "logoHeader": {
            "listed": "logo_header" in by_key,
            "exists": by_key.get("logo_header", {}).get("exists", False),
            "editable": by_key.get("logo_header", {}).get("meta", {}).get("editable", False),
        },
    }
    print(json.dumps({"refresh": summary}, sort_keys=True))

    time_status = request_json(opener, args.base + "/api/redbida/time-status")
    print(json.dumps({"timeStatus": {
        "ntpSynchronized": time_status.get("ntpSynchronized"),
        "nodeRedReadOnly": time_status.get("nodeRedReadOnly"),
        "driftThresholdSeconds": time_status.get("driftThresholdSeconds"),
    }}, sort_keys=True))

    if args.apply_key:
        if args.apply_value is None:
            raise RuntimeError("--apply-value is required with --apply-key")
        apply_result = request_json(opener, args.base + "/api/redbida/apply", "POST", {
            "changes": {args.apply_key: args.apply_value},
            "confirmed": False,
        })
        result = next((item for item in apply_result.get("results", []) if item.get("key") == args.apply_key), {})
        print(json.dumps({"apply": {
            "key": result.get("key"),
            "acknowledged": result.get("acknowledged"),
            "verified": result.get("verified"),
            "applied": result.get("applied"),
            "changed": result.get("changed"),
            "error": result.get("error", ""),
            "newValue": result.get("newValue"),
        }}, sort_keys=True))


if __name__ == "__main__":
    main()
