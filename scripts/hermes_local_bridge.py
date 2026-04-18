#!/usr/bin/env python3
import argparse
import json
import os
import sys
from urllib import error as urllib_error
from urllib import request as urllib_request


def parse_args():
    parser = argparse.ArgumentParser(description="Hermes local bridge client for OmniBull.")
    parser.add_argument("--base-url", default=os.environ.get("OMNIBULL_BASE_URL", "http://127.0.0.1:5409"))
    parser.add_argument("--api-key", default=os.environ.get("OMNIBULL_API_KEY", ""))
    parser.add_argument("--timeout", type=int, default=int(os.environ.get("OMNIBULL_TIMEOUT_MS", "15000")) // 1000 or 15)

    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("status", help="Read Hermes bridge status from OmniBull.")

    job_parser = subparsers.add_parser("job", help="Read a Hermes bridge job detail.")
    job_parser.add_argument("job_id")

    for name in ("chat", "run", "schedule"):
        command_parser = subparsers.add_parser(name, help=f"POST /api/hermes/{name if name != 'run' else 'task/run'}")
        command_parser.add_argument("--json", dest="json_payload")

    return parser.parse_args()


def load_payload(raw_json):
    if raw_json:
        return json.loads(raw_json)
    if sys.stdin.isatty():
        return {}
    text = sys.stdin.read().strip()
    return json.loads(text) if text else {}


def request_json(base_url, path, *, method="GET", payload=None, api_key="", timeout=15):
    base_url = str(base_url or "").rstrip("/")
    headers = {"Accept": "application/json"}
    data = None
    if api_key:
        headers["X-Omnibull-Key"] = api_key
    if payload is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(payload).encode("utf-8")
    req = urllib_request.Request(f"{base_url}{path}", method=method, headers=headers, data=data)
    try:
        with urllib_request.urlopen(req, timeout=max(1, timeout)) as response:
            raw = response.read().decode("utf-8")
            return response.status, json.loads(raw) if raw else {}
    except urllib_error.HTTPError as exc:
        raw = exc.read().decode("utf-8")
        try:
            parsed = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            parsed = {"error": raw or str(exc)}
        return exc.code, parsed


def main():
    args = parse_args()
    if args.command == "status":
        status, payload = request_json(
            args.base_url,
            "/api/hermes/status",
            api_key=args.api_key,
            timeout=args.timeout,
        )
    elif args.command == "job":
        status, payload = request_json(
            args.base_url,
            f"/api/hermes/jobs/{args.job_id}",
            api_key=args.api_key,
            timeout=args.timeout,
        )
    elif args.command == "chat":
        status, payload = request_json(
            args.base_url,
            "/api/hermes/chat",
            method="POST",
            payload=load_payload(args.json_payload),
            api_key=args.api_key,
            timeout=args.timeout,
        )
    elif args.command == "run":
        status, payload = request_json(
            args.base_url,
            "/api/hermes/task/run",
            method="POST",
            payload=load_payload(args.json_payload),
            api_key=args.api_key,
            timeout=args.timeout,
        )
    else:
        status, payload = request_json(
            args.base_url,
            "/api/hermes/task/schedule",
            method="POST",
            payload=load_payload(args.json_payload),
            api_key=args.api_key,
            timeout=args.timeout,
        )

    print(json.dumps(payload, ensure_ascii=False, indent=2))
    raise SystemExit(0 if 200 <= status < 300 else 1)


if __name__ == "__main__":
    main()
