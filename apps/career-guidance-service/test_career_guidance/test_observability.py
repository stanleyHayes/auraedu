from career_guidance_service.observability import (
    MAX_REQUEST_BODY_BYTES,
    PrometheusMiddleware,
    RequestBodyLimitMiddleware,
)
from fastapi import FastAPI, Request
from httpx import ASGITransport, AsyncClient


def metrics_app() -> FastAPI:
    app = FastAPI()

    @app.get("/items/{item_id}")
    async def item(item_id: str) -> dict[str, str]:
        return {"id": item_id}

    app.add_middleware(PrometheusMiddleware, service="career-guidance-service")
    return app


def bounded_app() -> FastAPI:
    app = FastAPI()

    @app.post("/body")
    async def body(request: Request) -> dict[str, int]:
        return {"bytes": len(await request.body())}

    app.add_middleware(RequestBodyLimitMiddleware)
    app.add_middleware(PrometheusMiddleware, service="career-guidance-service")
    return app


async def test_metrics_use_canonical_route_and_exclude_record_ids() -> None:
    app = metrics_app()
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        assert (await client.get("/items/private-record-id")).status_code == 200
        response = await client.get("/metrics")
    assert response.status_code == 200
    assert 'route="/items/{item_id}"' in response.text
    assert 'service="career-guidance-service"' in response.text
    assert "private-record-id" not in response.text


async def test_metrics_bearer_token(monkeypatch) -> None:
    monkeypatch.setenv("METRICS_BEARER_TOKEN", "metrics-test-secret")
    app = metrics_app()
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        assert (await client.get("/metrics")).status_code == 401
        response = await client.get(
            "/metrics", headers={"Authorization": "Bearer metrics-test-secret"}
        )
    assert response.status_code == 200
    assert "python_info" in response.text


async def test_request_body_limit_rejects_declared_and_streamed_overflow() -> None:
    async def oversized_stream():
        yield b"x" * MAX_REQUEST_BODY_BYTES
        yield b"y"

    app = bounded_app()
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        declared = await client.post("/body", content=b"x" * (MAX_REQUEST_BODY_BYTES + 1))
        streamed = await client.post("/body", content=oversized_stream())
        valid = await client.post("/body", content=b"{}")
    assert declared.status_code == 413
    assert declared.json()["code"] == "payload_too_large"
    assert streamed.status_code == 413
    assert streamed.json()["code"] == "payload_too_large"
    assert valid.status_code == 200
    assert valid.json() == {"bytes": 2}
