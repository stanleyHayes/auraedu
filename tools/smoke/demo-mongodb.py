#!/usr/bin/env python3
"""AURA-9.12 isolated full-demo Mongo replica-set smoke; requires Docker."""

import json
import os
import pathlib
import re
import secrets
import shutil
import subprocess
import tempfile
import time
import urllib.parse
from typing import Any, NoReturn, cast


def fail(message: str) -> NoReturn:
    raise AssertionError(message)


def check(condition: bool, message: str) -> None:
    if not condition:
        fail(message)


class DemoSmoke:
    def __init__(self) -> None:
        self.docker_binary = shutil.which("docker") or ""
        check(bool(self.docker_binary), "Docker executable is required")
        suffix = secrets.token_hex(4)
        self.network, self.mongo, self.demo = [
            f"auraedu-smoke-{suffix}-{x}" for x in ("net", "mongo", "demo")
        ]
        self.capture = f"auraedu-smoke-{suffix}-smtp"
        self.image = os.environ.get("DEMO_IMAGE", "auraedu-demo:mongo-local")
        self.password, self.internal, self.signing = [secrets.token_urlsafe(32) for _ in range(3)]

    def run(self, *args: str) -> str:
        return subprocess.check_output(args, text=True, timeout=120).strip()  # noqa: S603 - fixed Docker argv, no host shell

    def shell(self, js: str) -> str:
        return self.run(self.docker_binary, "exec", self.mongo, "mongosh", "--quiet", "--eval", js)

    def local(self, port: int, path: str, data: dict[str, Any] | None = None) -> dict[str, Any]:
        args = [
            self.docker_binary,
            "exec",
            self.demo,
            "wget",
            "-q",
            "-O",
            "-",
            "--header=X-Actor-User: smoke-platform-admin",
            "--header=X-Actor-Role: platform_super_admin",
            "--header=Content-Type: application/json",
        ]
        if data is not None:
            args += ["--post-data=" + json.dumps(data)]
        return cast(dict[str, Any], json.loads(self.run(*args, f"http://127.0.0.1:{port}" + path)))

    def request(
        self,
        path: str,
        data: dict[str, Any] | None = None,
        token: str | None = None,
        *,
        tenant: str = "smoke-a",
        expected: int = 200,
    ) -> dict[str, Any]:
        result = cast(
            dict[str, Any],
            json.loads(
                self.run(
                    self.docker_binary,
                    "exec",
                    "-e",
                    "SMOKE_HTTP_PATH=" + path,
                    "-e",
                    "SMOKE_HTTP_TENANT=" + tenant,
                    "-e",
                    "SMOKE_HTTP_TOKEN=" + (token or ""),
                    "-e",
                    "SMOKE_HTTP_BODY=" + (json.dumps(data) if data is not None else ""),
                    "-e",
                    "SMOKE_HTTP_IDEMPOTENCY=" + secrets.token_hex(16),
                    self.demo,
                    "demo-smtp-capture",
                    "request",
                )
            ),
        )
        status, body = result["status"], result["body"]
        if status != expected:
            fail(f"{path}: expected {expected}, got {status}: {body[:300]!r}")
        return cast(dict[str, Any], json.loads(body)) if body else {}

    def ready(self) -> None:
        for _ in range(180):
            try:
                self.request("/ready")
            except OSError, AssertionError, subprocess.CalledProcessError:
                state = json.loads(self.run(self.docker_binary, "inspect", self.demo))[0]["State"]
                check(bool(state["Running"]), "demo exited before readiness")
                time.sleep(1)
            else:
                return
        fail("gateway never ready")

    def bootstrap(self) -> None:
        self.run(self.docker_binary, "network", "create", "--internal", self.network)
        self.run(
            self.docker_binary,
            "run",
            "-d",
            "--name",
            self.mongo,
            "--network",
            self.network,
            "mongo:8.0",
            "--replSet",
            "rs0",
            "--bind_ip_all",
        )
        for _ in range(90):
            try:
                self.shell(
                    f'rs.initiate({{_id:"rs0",members:[{{_id:0,host:"{self.mongo}:27017"}}]}})'
                )
                break
            except subprocess.CalledProcessError:
                time.sleep(1)
        for _ in range(90):
            if self.shell("db.hello().isWritablePrimary") == "true":
                break
            time.sleep(1)
        else:
            fail("replica set not primary")
        self.shell(
            """const d=db.getSiblingDB("auraedu_demo_tenant");
    for(const code of ["smoke-a","smoke-b"]) {
          d.tenants.insertOne({_id:code,code,name:code,status:"active",
          plan:"professional",created_at:new Date(),
          updated_at:new Date()});
          d.tenant_features.insertOne({_id:code,tenant_code:code,flags:{
            student_management:{is_enabled:true},
            staff_management:{is_enabled:false},
            analytics:{is_enabled:true},
            notifications:{is_enabled:false}}}); }"""
        )
        self.run(
            self.docker_binary,
            "run",
            "-d",
            "--name",
            self.capture,
            "--network",
            self.network,
            "--entrypoint",
            "demo-smtp-capture",
            self.image,
        )
        self.run(
            self.docker_binary,
            "run",
            "-d",
            "--name",
            self.demo,
            "--network",
            self.network,
            "--memory",
            "512m",
            "-e",
            "DATABASE_DRIVER=mongodb",
            "-e",
            f"MONGODB_URI=mongodb://{self.mongo}:27017/?replicaSet=rs0",
            "-e",
            f"INTERNAL_SERVICE_TOKEN={self.internal}",
            "-e",
            f"JWT_SIGNING_KEY={self.signing}",
            "-e",
            "NOTIFICATION_PROVIDER=smtp",
            "-e",
            f"SMTP_HOST={self.capture}",
            "-e",
            "SMTP_PORT=2525",
            "-e",
            "SMTP_ALLOW_INSECURE=true",
            "-e",
            "SMTP_FROM_EMAIL=smoke@example.test",
            self.image,
        )
        self.ready()

    def verify_runtime(self) -> None:
        listeners = self.run(self.docker_binary, "exec", self.demo, "netstat", "-ltn")
        for line in listeners.splitlines()[2:]:
            address = line.split()[3]
            host, port = address.rsplit(":", 1)
            check(
                host.startswith("127.") or host == "::1" or port == "8080",
                "non-gateway listener exposed beyond loopback",
            )
        processes = self.run(self.docker_binary, "exec", self.demo, "ps", "-o", "args")
        workers = [
            line
            for line in processes.splitlines()
            if line.strip().startswith("/usr/local/bin/") and line.strip().endswith(" worker")
        ]
        expected_workers = 18
        check(len(workers) == expected_workers, "expected all 18 workers to run")
        memory = self.run(
            self.docker_binary, "stats", "--no-stream", "--format", "{{.MemUsage}}", self.demo
        )
        print(
            f"Full demo runtime: {len(workers)} workers; memory {memory}; "
            "loopback isolation verified"
        )

    def onboarding_flow(self) -> None:
        self.local(
            8100,
            "/api/v1/billing/plans",
            {
                "name": "Smoke Growth",
                "code": "growth",
                "price_cents": 0,
                "currency": "GHS",
                "billing_interval": "monthly",
                "features": [],
            },
        )
        onboarding = self.request(
            "/api/v1/public/onboarding-requests",
            {
                "school_name": "Smoke Activation",
                "administrator_name": "Smoke Admin",
                "email": "activation@example.test",
                "country_code": "GH",
                "plan": "growth",
                "privacy_notice_version": "2026-07",
                "accepted_terms": True,
                "website": "",
            },
            tenant="",
            expected=202,
        )
        approval = self.local(
            8082,
            "/api/v1/super-admin/onboarding-requests/" + onboarding["request_id"] + "/approve",
            {"tenant_code": "activation-smoke"},
        )
        check(approval["status"] == "approved", "onboarding approval failed")
        invite = None
        for _ in range(90):
            raw = self.run(
                self.docker_binary,
                "exec",
                self.demo,
                "wget",
                "-q",
                "-O",
                "-",
                f"http://{self.capture}:8025/messages",
            )
            for message in cast(list[str] | None, json.loads(raw)) or []:
                decoded = message
                if "activation@example.test" in decoded:
                    match = re.search("#token=([^\\s<>]+)", decoded)
                    if match:
                        invite = urllib.parse.unquote(match.group(1))
                        break
            if invite:
                break
            time.sleep(1)
        if not invite:
            fail("onboarding invite was not delivered to local SMTP")
        accepted = self.local(
            8081,
            "/api/v1/users/invites/" + invite + "/accept",
            {"name": "Smoke Admin", "password": self.password},
        )
        check(accepted["tenant_id"] == "activation-smoke", "invite tenant mismatch")
        activated = self.request(
            "/api/v1/auth/login",
            {"email": "activation@example.test", "password": self.password},
            tenant="activation-smoke",
        )
        check(bool(activated["access_token"]), "activated login failed")
        for _ in range(60):
            provisioned = self.shell(
                'db.getSiblingDB("auraedu_demo_billing").billing_subscriptions.countDocuments({tenant_id:"activation-smoke"})'
            )
            if int(provisioned) > 0:
                break
            time.sleep(1)
        else:
            fail("billing worker failed to provision subscription")
        print(
            "PASS: onboarding approval, local SMTP invite, acceptance, "
            "activated login, billing worker subscription"
        )

    def seed_logins(self) -> None:
        for tenant in ("smoke-a", "smoke-b"):
            self.run(
                self.docker_binary,
                "exec",
                "-e",
                "MONGODB_DATABASE=auraedu_demo_identity",
                "-e",
                f"DEMO_TENANT_ID={tenant}",
                "-e",
                f"DEMO_USER_EMAIL={tenant}@example.test",
                "-e",
                f"DEMO_USER_PASSWORD={self.password}",
                "-e",
                "DEMO_USER_ROLE=school_admin",
                self.demo,
                "identity-service",
                "seed-demo",
            )

    def records_flow(self) -> str:
        login = self.request(
            "/api/v1/auth/login", {"email": "smoke-a@example.test", "password": self.password}
        )
        token = login["access_token"]
        self.refresh = login["refresh_token"]
        student = self.request(
            "/api/v1/students", {"first_name": "Smoke", "last_name": "Student"}, token, expected=201
        )
        sid = student["id"]
        other = self.request(
            "/api/v1/auth/login",
            {"email": "smoke-b@example.test", "password": self.password},
            tenant="smoke-b",
        )["access_token"]
        self.request("/api/v1/students/" + sid, token=other, tenant="smoke-b", expected=404)
        disabled = self.request("/api/v1/staff", {}, token=token, expected=403)
        if "feature_disabled" not in json.dumps(disabled):
            fail("staff denied for wrong reason")
        for _ in range(60):
            pending = self.shell(
                f'''const x=db.getSiblingDB("auraedu_demo_student").students.findOne(
                {{_id:"{sid}"}});
                print(x ? (x._pending_events||[]).length : -1)'''
            )
            if pending == "0":
                break
            time.sleep(1)
        else:
            fail("student outbox did not drain")
        for _ in range(60):
            consumed = self.shell(
                'db.getSiblingDB("auraedu_demo_audit").audit_logs.countDocuments({tenant_id:"smoke-a",event_type:"student.created.v1"})'
            )
            if int(consumed) > 0:
                break
            time.sleep(1)
        else:
            fail("audit worker did not persist student event")
        return str(sid)

    def sessions_flow(self, sid: str) -> None:
        rotated = self.request("/api/v1/auth/refresh", {"refresh_token": self.refresh})
        self.request("/api/v1/auth/refresh", {"refresh_token": self.refresh}, expected=401)
        self.request(
            "/api/v1/auth/refresh", {"refresh_token": rotated["refresh_token"]}, expected=401
        )
        active = self.request(
            "/api/v1/auth/login", {"email": "smoke-a@example.test", "password": self.password}
        )
        self.request(
            "/api/v1/auth/logout",
            {"refresh_token": active["refresh_token"]},
            active["access_token"],
        )
        self.request(
            "/api/v1/auth/refresh", {"refresh_token": active["refresh_token"]}, expected=401
        )
        self.run(self.docker_binary, "restart", "--time", "25", self.demo)
        self.ready()
        active = self.request(
            "/api/v1/auth/login", {"email": "smoke-a@example.test", "password": self.password}
        )
        self.request("/api/v1/students/" + sid, token=active["access_token"])

    def supervisor_flow(self) -> None:
        self.run(
            self.docker_binary,
            "exec",
            self.demo,
            "sh",
            "-c",
            r"""for p in /proc/[0-9]*; do
              [ -r "$p/cmdline" ] || continue
              cmd=$(tr '\000' ' ' < "$p/cmdline")
              case "$cmd" in
                "/usr/local/bin/student-service worker"*)
                  kill -TERM "${p##*/}"; exit 0 ;;
              esac
            done
            exit 1""",
        )
        for _ in range(30):
            state = json.loads(self.run(self.docker_binary, "inspect", self.demo))[0]["State"]
            if not state["Running"]:
                check(state["ExitCode"] != 0, "supervisor reported success after child failure")
                break
            time.sleep(1)
        else:
            fail("supervisor did not exit after worker failure")
        print(
            "PASS: full demo readiness, real login, write, tenant isolation, disabled feature, "
            "outbox drain, refresh replay, logout revocation, restart persistence"
        )

    def execute(self) -> None:
        try:
            self.bootstrap()
            self.verify_runtime()
            self.onboarding_flow()
            self.seed_logins()
            sid = self.records_flow()
            self.sessions_flow(sid)
            self.supervisor_flow()
        except Exception:
            logs = pathlib.Path(tempfile.mkdtemp(prefix="auraedu-demo-smoke-"))
            for name in (self.demo, self.mongo, self.capture):
                with (logs / (name + ".log")).open("w") as output:
                    subprocess.run(  # noqa: S603 - owned random Docker resources only
                        [self.docker_binary, "logs", "--tail", "180", name],
                        stdout=output,
                        stderr=subprocess.STDOUT,
                        timeout=30,
                        check=False,
                    )
            print(f"Smoke diagnostics: {logs}")
            raise
        finally:
            if os.environ.get("KEEP_DEMO_SMOKE") == "1":
                print(
                    f"Kept containers {self.demo}, {self.mongo}; "
                    f"network {self.network}; capture {self.capture}"
                )
            else:
                subprocess.run(  # noqa: S603 - owned random Docker resources only
                    [
                        self.docker_binary,
                        "container",
                        "remove",
                        "--force",
                        self.demo,
                        self.mongo,
                        self.capture,
                    ],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    check=False,
                    timeout=120,
                )
                subprocess.run(  # noqa: S603 - owned random Docker resources only
                    [self.docker_binary, "network", "remove", self.network],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    check=False,
                    timeout=120,
                )


if __name__ == "__main__":
    DemoSmoke().execute()
