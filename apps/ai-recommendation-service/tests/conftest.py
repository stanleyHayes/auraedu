"""Test fixtures for the AI Recommendation Service."""

import os
from collections.abc import AsyncGenerator, Iterator

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient

# Force an async SQLite database for tests before importing the app.
os.environ.setdefault("AI_REC_DATABASE_URL", "sqlite+aiosqlite:///./ai_rec_test.db")
os.environ.setdefault("AI_REC_NATS_HOST", "")

import ai_recommendation_service.main as main_module
from ai_recommendation_service import feature_flags
from ai_recommendation_service.db import engine
from ai_recommendation_service.events.publisher import RecordingPublisher
from ai_recommendation_service.main import app
from ai_recommendation_service.models import Base

main_module.app_publisher = RecordingPublisher()


@pytest_asyncio.fixture(autouse=True)
async def setup_db() -> AsyncGenerator[None]:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.drop_all)


@pytest_asyncio.fixture
async def client() -> AsyncGenerator[AsyncClient]:
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        yield ac


@pytest.fixture
def tenant_id() -> str:
    return "upshs"


@pytest.fixture(autouse=True)
def enabled_test_tenant(tenant_id: str) -> Iterator[None]:
    original = feature_flags.gate.is_enabled(tenant_id, "ai_recommendations")
    feature_flags.gate.set_enabled(tenant_id, "ai_recommendations", True)
    yield
    feature_flags.gate.set_enabled(tenant_id, "ai_recommendations", original)


@pytest.fixture
def actor_headers() -> dict[str, str]:
    return {
        "X-Actor-User": "11111111-1111-1111-1111-111111111111",
        "X-Actor-Role": "teacher",
        "X-Actor-Tenant": "upshs",
        "X-Actor-Permissions": "ai.view_recommendations,ai.approve_recommendations",
    }
