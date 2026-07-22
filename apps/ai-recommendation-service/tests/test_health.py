"""Health endpoint tests."""

import pytest
from ai_recommendation_service.api import routes
from httpx import AsyncClient


@pytest.mark.asyncio
async def test_health(client: AsyncClient) -> None:
    response = await client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


@pytest.mark.asyncio
async def test_readiness_checks_database(client: AsyncClient) -> None:
    response = await client.get("/ready")
    assert response.status_code == 200
    assert response.json() == {"status": "ready"}


@pytest.mark.asyncio
async def test_readiness_fails_closed_without_database(
    client: AsyncClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    async def unavailable() -> bool:
        return False

    monkeypatch.setattr(routes, "database_ready", unavailable)
    response = await client.get("/ready")
    assert response.status_code == 503
    assert response.json() == {
        "code": "not_ready",
        "message": "Database dependency is unavailable",
    }
