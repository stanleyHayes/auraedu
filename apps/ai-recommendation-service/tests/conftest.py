"""Test fixtures for the AI Recommendation Service."""

import os
import uuid
from collections.abc import AsyncGenerator

import pytest
import pytest_asyncio

# Force an async SQLite database for tests before importing the app.
os.environ.setdefault("AI_REC_DATABASE_URL", "sqlite+aiosqlite:///./ai_rec_test.db")
os.environ.setdefault("AI_REC_NATS_HOST", "")

import ai_recommendation_service.main as main_module  # noqa: E402
from ai_recommendation_service.db import engine  # noqa: E402
from ai_recommendation_service.events.publisher import RecordingPublisher  # noqa: E402
from ai_recommendation_service.main import app  # noqa: E402
from ai_recommendation_service.models import Base  # noqa: E402

main_module.app_publisher = RecordingPublisher()


@pytest_asyncio.fixture(autouse=True)
async def setup_db() -> AsyncGenerator[None]:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.drop_all)


@pytest_asyncio.fixture
async def client() -> AsyncGenerator:
    from httpx import ASGITransport, AsyncClient

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        yield ac


@pytest.fixture
def tenant_id() -> str:
    return str(uuid.uuid4())


@pytest.fixture
def actor_headers() -> dict[str, str]:
    return {
        "X-Actor-User": "11111111-1111-1111-1111-111111111111",
        "X-Actor-Role": "teacher",
        "X-Actor-Tenant": "test-tenant",
        "X-Actor-Permissions": "ai.view_recommendations,ai.approve_recommendations",
    }
