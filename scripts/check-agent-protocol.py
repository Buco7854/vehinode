#!/usr/bin/env python3
"""Exercise the public VehiNode device API with a custom agent version."""

import argparse
import json
import sys
from datetime import UTC, datetime
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen
from uuid import uuid4


def request_json(
    server: str,
    path: str,
    *,
    payload: dict[str, Any] | None = None,
    credential: str | None = None,
) -> dict[str, Any]:
    data = json.dumps(payload, separators=(",", ":")).encode() if payload else None
    request = Request(server + path, data=data, method="POST" if data else "GET")
    request.add_header("Accept", "application/json")
    if data:
        request.add_header("Content-Type", "application/json")
    if credential:
        request.add_header("Authorization", f"Device {credential}")
    try:
        with urlopen(request, timeout=30) as response:  # noqa: S310 - validated URL
            result = json.load(response)
    except HTTPError as exc:
        detail = exc.read().decode(errors="replace")
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {detail}") from exc
    except URLError as exc:
        raise RuntimeError(f"cannot reach {server}: {exc.reason}") from exc
    if not isinstance(result, dict):
        raise RuntimeError(f"{path} returned a non-object response")
    return result


def server_origin(value: str, allow_http: bool) -> str:
    parsed = urlparse(value)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError("--server must be an HTTP(S) origin")
    if parsed.path not in {"", "/"} or parsed.params or parsed.query or parsed.fragment:
        raise ValueError("--server must not contain a path, query, or fragment")
    if parsed.scheme == "http" and not allow_http:
        raise ValueError("plain HTTP requires --allow-http")
    return value.rstrip("/")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Enroll a custom agent and verify the VehiNode device API. "
            "The agent and server major versions must match."
        )
    )
    parser.add_argument("--server", required=True, help="VehiNode origin")
    parser.add_argument("--token", required=True, help="one-time enrollment token")
    parser.add_argument(
        "--agent-version",
        required=True,
        help="agent SemVer; its major version must match the server",
    )
    parser.add_argument(
        "--allow-http", action="store_true", help="allow credentials over plain HTTP"
    )
    return parser.parse_args()


def main() -> int:
    arguments = parse_args()
    try:
        server = server_origin(arguments.server, arguments.allow_http)
        enrolled = request_json(
            server,
            "/api/v1/device/enroll",
            payload={
                "token": arguments.token,
                "agent_version": arguments.agent_version,
            },
        )
        credential = enrolled.get("credential")
        if not isinstance(credential, str) or not credential:
            raise RuntimeError("enrollment response has no device credential")
        configuration = request_json(server, "/api/v1/device/config", credential=credential)
        if not isinstance(configuration.get("version"), int):
            raise RuntimeError("configuration has no integer version")
        if not isinstance(configuration.get("sampling"), dict):
            raise RuntimeError("configuration has no sampling policy")
        if not isinstance(configuration.get("upload"), dict):
            raise RuntimeError("configuration has no upload policy")

        sample_id = str(uuid4())
        batch = {
            "boot_id": str(uuid4()),
            "samples": [
                {
                    "id": sample_id,
                    "sequence": 1,
                    "recorded_at": datetime.now(UTC).isoformat(),
                    "metrics": {"diagnostic.conformance": True},
                    "device": {"agent.conformance": True},
                }
            ],
        }
        accepted = request_json(
            server,
            "/api/v1/device/telemetry/batch",
            payload=batch,
            credential=credential,
        )
        duplicate = request_json(
            server,
            "/api/v1/device/telemetry/batch",
            payload=batch,
            credential=credential,
        )
        if accepted.get("accepted") != [sample_id]:
            raise RuntimeError(f"first upload was not accepted: {accepted!r}")
        if duplicate.get("duplicates") != [sample_id]:
            raise RuntimeError(f"retry was not idempotent: {duplicate!r}")
    except (RuntimeError, ValueError) as exc:
        print(f"Agent protocol check failed: {exc}", file=sys.stderr)
        return 1

    print("Custom agent API check passed.")
    print(f"Device: {enrolled.get('device_id')}")
    print("The test credential was not persisted; revoke this test tracker in Devices.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
