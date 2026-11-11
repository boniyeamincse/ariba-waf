#!/usr/bin/env python3
"""Test the local management image in an isolated Compose project and volume.

Build first: docker build -f Dockerfile.api -t ariba-shield-management-api:local .
Run: python3 tests/integration/management_container.py
Only this test's uniquely named containers, network and volume are removed.
"""

import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import urllib.error
import urllib.request
import uuid


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def main():
    root = Path(__file__).resolve().parents[2]
    project = "ariba-api-test-" + uuid.uuid4().hex[:12]
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    env = dict(os.environ, ARIBA_API_PORT=str(port))
    compose = ["docker", "compose", "-f", str(root / "compose.yaml"), "-p", project]
    origin = f"http://127.0.0.1:{port}"
    cookies = http.cookiejar.CookieJar()
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies)
    )

    def command(args, check=True):
        result = subprocess.run(args, cwd=root, env=env, text=True, capture_output=True, timeout=120)
        if check and result.returncode:
            raise RuntimeError(f"Command failed: {' '.join(args)}\n{result.stderr}")
        return result

    def request(path, method="GET", body=None, csrf=None, etag=None, key=None):
        headers = {"Content-Type": "application/json", "Origin": origin}
        for name, value in (("X-CSRF-Token", csrf), ("If-Match", etag), ("Idempotency-Key", key)):
            if value is not None:
                headers[name] = value
        req = urllib.request.Request(
            origin + path, method=method, headers=headers,
            data=json.dumps(body).encode() if body is not None else None,
        )
        try:
            response = opener.open(req, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            return response.status, json.loads(raw) if raw else None, response.headers

    def expect(path, status=200, **kwargs):
        actual, body, headers = request(path, **kwargs)
        require(actual == status, f"{path}: got HTTP {actual}, expected {status}")
        return body, headers

    def inspect_container():
        container_id = command(compose + ["ps", "-q", "management-api"]).stdout.strip()
        info = json.loads(command(["docker", "inspect", container_id]).stdout)[0]
        require(info["Config"]["User"] == "65532:65532", "runtime user must be non-root")
        require(info["HostConfig"]["ReadonlyRootfs"], "root filesystem must be read-only")
        require("ALL" in info["HostConfig"]["CapDrop"], "capabilities were not dropped")
        require("no-new-privileges:true" in info["HostConfig"]["SecurityOpt"], "privilege restriction missing")
        require(info["HostConfig"]["Memory"] == 512 * 1024 * 1024, "memory limit missing")
        require(info["HostConfig"]["NanoCpus"] == 1_000_000_000, "CPU limit missing")
        require(info["HostConfig"]["PidsLimit"] == 64, "PID limit missing")
        temporary = info["HostConfig"]["Tmpfs"].get("/tmp", "")
        require("noexec" in temporary and "nosuid" in temporary, "temporary filesystem restrictions missing")
        require(info["State"]["Health"]["Status"] == "healthy", "liveness healthcheck failed")
        bindings = info["HostConfig"]["PortBindings"]["9000/tcp"]
        require(bindings == [{"HostIp": "127.0.0.1", "HostPort": str(port)}], "port must be host-loopback-only")
        mounts = [m for m in info["Mounts"] if m["Destination"] == "/data"]
        require(len(mounts) == 1 and mounts[0]["Type"] == "volume" and mounts[0]["RW"], "persistent volume missing")
        return container_id, mounts[0]["Name"]

    try:
        command(compose + ["up", "-d", "--no-build", "--wait", "--wait-timeout", "90", "management-api"])
        before_id, before_volume = inspect_container()
        with opener.open(origin + "/", timeout=10) as page:
            require(page.status == 200 and page.headers.get_content_type() == "text/html", "root status page missing")
            require("Ariba Shield" in page.read().decode(), "unexpected root page")
        expect("/health/live")
        expect("/health/ready", 503)
        body, _ = expect("/api/v1/setup/status")
        require(body["data"]["enrollmentRequired"], "test volume was not fresh")
        token = command(compose + ["exec", "-T", "management-api", "/ariba", "-db", "/data/db/ariba.db", "-enrollment-token"]).stdout.strip()
        body, _ = expect("/api/v1/auth/csrf")
        csrf = body["data"]["csrfToken"]
        password = secrets.token_urlsafe(24)
        expect("/api/v1/setup/enroll", 201, method="POST", csrf=csrf, key="container-enrollment", body={
            "enrollmentToken": token, "username": "container-admin", "displayName": "Container test", "password": password,
        })
        expect("/api/v1/auth/login", method="POST", csrf=csrf, body={"username": "container-admin", "password": password})
        body, _ = expect("/api/v1/auth/csrf")
        csrf = body["data"]["csrfToken"]
        _, headers = expect("/api/v1/settings/proxy")
        expect("/api/v1/settings/proxy", method="PATCH", csrf=csrf, etag=headers["ETag"], body={"allowedCidrs": ["10.24.20.0/24"]})
        body, _ = expect("/api/v1/policies", 201, method="POST", csrf=csrf, body={"name": "Container detection"})
        policy = body["data"]["id"]
        body, site_headers = expect("/api/v1/websites", 201, method="POST", csrf=csrf, body={
            "name": "Container site", "domains": ["container.example.test"], "policyId": policy,
            "upstreams": [{"scheme": "http", "host": "10.24.20.50", "port": 8080}],
        })
        site_id = body["data"]["id"]
        require(body["data"]["appliedRevisionId"] is None, "desired site reported as applied")
        print("PASS: non-root/read-only runtime, loopback port, named volume, liveness, setup/login and site/policy APIs", flush=True)

        command(compose + ["up", "-d", "--no-build", "--force-recreate", "--wait", "--wait-timeout", "90", "management-api"])
        after_id, after_volume = inspect_container()
        require(before_id != after_id and before_volume == after_volume, "container recreation did not reuse its volume")
        body, _ = expect("/api/v1/auth/me")
        require(body["data"]["user"]["username"] == "container-admin", "account/session did not persist")
        body, headers = expect("/api/v1/websites/" + site_id)
        require(headers["ETag"] == site_headers["ETag"] and body["data"]["policyId"] == policy, "desired site did not persist")
        body, _ = expect("/api/v1/configuration/status")
        require(body["data"]["desiredGeneration"] == 3 and body["data"]["appliedRevisionId"] is None, "generation/applied state did not persist")
        result = command(compose + ["exec", "-T", "management-api", "/ariba", "-db", "/data/db/ariba.db", "-enrollment-token"], check=False)
        require(result.returncode != 0, "bootstrap remained available after setup")
        expect("/api/v1/auth/logout", 204, method="POST", csrf=csrf)
        expect("/api/v1/auth/me", 401)
        print("PASS: container replacement preserves accounts, sessions, policies, sites and configuration generation; logout revokes session", flush=True)
    finally:
        # This project/volume was created with a unique test-only name above.
        command(compose + ["down", "--volumes", "--remove-orphans"])


if __name__ == "__main__":
    main()
