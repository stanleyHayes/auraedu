"""Low-cardinality Prometheus middleware for AuraEDU FastAPI services."""

from __future__ import annotations

import hmac
import os
import time
from collections.abc import Awaitable, Callable
from typing import Any

from prometheus_client import (
    CONTENT_TYPE_LATEST,
    CollectorRegistry,
    Counter,
    Gauge,
    GCCollector,
    Histogram,
    PlatformCollector,
    ProcessCollector,
    generate_latest,
)

ASGIApp = Callable[
    [
        dict[str, Any],
        Callable[[], Awaitable[dict[str, Any]]],
        Callable[[dict[str, Any]], Awaitable[None]],
    ],
    Awaitable[None],
]

MAX_REQUEST_BODY_BYTES = 1 << 20


class _RequestBodyTooLargeError(Exception):
    pass


class RequestBodyLimitMiddleware:
    """Reject declared and streamed request bodies above the service ceiling."""

    def __init__(self, app: ASGIApp, max_bytes: int = MAX_REQUEST_BODY_BYTES) -> None:
        self.app = app
        self.max_bytes = max_bytes

    async def __call__(
        self,
        scope: dict[str, Any],
        receive: Callable[[], Awaitable[dict[str, Any]]],
        send: Callable[[dict[str, Any]], Awaitable[None]],
    ) -> None:
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return
        headers = {key.lower(): value for key, value in scope.get("headers", [])}
        declared = headers.get(b"content-length", b"").decode("ascii", errors="ignore")
        if declared.isdigit() and int(declared) > self.max_bytes:
            await _payload_too_large(send)
            return

        received = 0

        async def bounded_receive() -> dict[str, Any]:
            nonlocal received
            message = await receive()
            if message.get("type") == "http.request":
                received += len(message.get("body", b""))
                if received > self.max_bytes:
                    raise _RequestBodyTooLargeError
            return message

        try:
            await self.app(scope, bounded_receive, send)
        except _RequestBodyTooLargeError:
            await _payload_too_large(send)


class PrometheusMiddleware:
    """Expose /metrics and measure canonical FastAPI route patterns."""

    def __init__(self, app: ASGIApp, service: str) -> None:
        self.app = app
        self.service = service
        self.registry = CollectorRegistry()
        GCCollector(registry=self.registry)
        PlatformCollector(registry=self.registry)
        ProcessCollector(registry=self.registry)
        self.requests = Counter(
            "auraedu_http_requests_total",
            "Total AuraEDU HTTP requests by service, method, canonical route and status.",
            ("service", "method", "route", "status"),
            registry=self.registry,
        )
        self.duration = Histogram(
            "auraedu_http_request_duration_seconds",
            "AuraEDU HTTP request duration by service, method and canonical route.",
            ("service", "method", "route"),
            buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 0.75, 1, 1.5, 2.5, 5, 10),
            registry=self.registry,
        )
        self.in_flight = Gauge(
            "auraedu_http_requests_in_flight",
            "Current AuraEDU HTTP requests in flight by service.",
            ("service",),
            registry=self.registry,
        )

    async def __call__(
        self,
        scope: dict[str, Any],
        receive: Callable[[], Awaitable[dict[str, Any]]],
        send: Callable[[dict[str, Any]], Awaitable[None]],
    ) -> None:
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return
        if scope.get("path") == "/metrics":
            await self._serve_metrics(scope, send)
            return

        method = str(scope.get("method", "UNKNOWN")).upper()
        status = 500
        started = time.perf_counter()
        self.in_flight.labels(self.service).inc()

        async def observe_start(message: dict[str, Any]) -> None:
            nonlocal status
            if message.get("type") == "http.response.start":
                status = int(message.get("status", 500))
            await send(message)

        try:
            await self.app(scope, receive, observe_start)
        finally:
            self.in_flight.labels(self.service).dec()
            route_object = scope.get("route")
            route = str(getattr(route_object, "path", "unmatched"))
            self.requests.labels(self.service, method, route, str(status)).inc()
            self.duration.labels(self.service, method, route).observe(time.perf_counter() - started)

    async def _serve_metrics(
        self,
        scope: dict[str, Any],
        send: Callable[[dict[str, Any]], Awaitable[None]],
    ) -> None:
        expected = os.getenv("METRICS_BEARER_TOKEN", "").strip()
        if expected and not _authorized(scope, expected):
            await _respond(
                send, 401, b"unauthorized\n", b"text/plain; charset=utf-8", authenticate=True
            )
            return
        await _respond(send, 200, generate_latest(self.registry), CONTENT_TYPE_LATEST.encode())


def _authorized(scope: dict[str, Any], expected: str) -> bool:
    headers = {key.lower(): value for key, value in scope.get("headers", [])}
    supplied = headers.get(b"authorization", b"").decode("latin-1")
    prefix = "Bearer "
    return supplied.startswith(prefix) and hmac.compare_digest(
        supplied[len(prefix) :].strip(), expected
    )


async def _respond(
    send: Callable[[dict[str, Any]], Awaitable[None]],
    status: int,
    body: bytes,
    content_type: bytes,
    *,
    authenticate: bool = False,
) -> None:
    headers = [(b"content-type", content_type), (b"content-length", str(len(body)).encode())]
    if authenticate:
        headers.append((b"www-authenticate", b'Bearer realm="metrics"'))
    await send({"type": "http.response.start", "status": status, "headers": headers})
    await send({"type": "http.response.body", "body": body})


async def _payload_too_large(
    send: Callable[[dict[str, Any]], Awaitable[None]],
) -> None:
    await _respond(
        send,
        413,
        b'{"code":"payload_too_large","message":"request body too large"}',
        b"application/json",
    )
