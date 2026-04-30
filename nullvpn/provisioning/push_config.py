#!/usr/bin/env python3
"""
push_config.py
Backend helper: pushes a per-user sing-box outbound config to
nullvpn-registry/configs/{device_id}.json via GitHub REST API.

Called by nullvpn-backend (TypeScript) as a subprocess, or directly
from the GitHub Actions provision.yml workflow after generating config.

Usage:
  python3 push_config.py <device_id> <config_json_string>

Env vars required:
  REGISTRY_GITHUB_PAT   - Fine-grained PAT: contents:write on nullvpn-registry
  REGISTRY_GITHUB_REPO  - e.g. nullvpnnet/nullvpn-registry (default)
"""

import sys
import os
import json
import base64
import hashlib
import urllib.request
import urllib.error

REPO  = os.environ.get("REGISTRY_GITHUB_REPO", "nullvpnnet/nullvpn-registry")
PAT   = os.environ.get("REGISTRY_GITHUB_PAT", "")
API   = "https://api.github.com"


def get_file_sha(path: str) -> str | None:
    url = f"{API}/repos/{REPO}/contents/{path}"
    req = urllib.request.Request(url, headers={
        "Authorization": f"Bearer {PAT}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    try:
        with urllib.request.urlopen(req) as resp:
            data = json.loads(resp.read())
            return data.get("sha")
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return None
        raise


def push_file(path: str, content_str: str, message: str) -> bool:
    if not PAT:
        print("ERROR: REGISTRY_GITHUB_PAT not set", file=sys.stderr)
        return False

    encoded = base64.b64encode(content_str.encode("utf-8")).decode("ascii")
    existing_sha = get_file_sha(path)

    body: dict = {"message": message, "content": encoded}
    if existing_sha:
        body["sha"] = existing_sha

    url = f"{API}/repos/{REPO}/contents/{path}"
    req = urllib.request.Request(
        url,
        data=json.dumps(body).encode("utf-8"),
        method="PUT",
        headers={
            "Authorization": f"Bearer {PAT}",
            "Content-Type": "application/json",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status in (200, 201)
    except urllib.error.HTTPError as e:
        print(f"ERROR: GitHub push failed: {e.code} {e.read().decode()}", file=sys.stderr)
        return False


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <device_id> <config_json>", file=sys.stderr)
        sys.exit(1)

    device_id   = sys.argv[1]
    config_json = sys.argv[2]

    # Validate it's real JSON
    try:
        json.loads(config_json)
    except json.JSONDecodeError as e:
        print(f"ERROR: Invalid config JSON: {e}", file=sys.stderr)
        sys.exit(1)

    path    = f"configs/{device_id}.json"
    message = f"provision: {device_id}"

    print(f"Pushing config to {REPO}/{path} ...")
    ok = push_file(path, config_json, message)

    if ok:
        print(f"OK: {path} pushed successfully")
        sys.exit(0)
    else:
        print(f"FAIL: Could not push {path}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
