"""Test configuration for AI Prediction Service."""

import os
from collections.abc import AsyncGenerator
from pathlib import Path

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession

os.environ.setdefault("AI_PRED_DATABASE_URL", "sqlite+aiosqlite:///:memory:")
os.environ.setdefault("AI_PRED_NATS_HOST", "")
os.environ.setdefault(
    "AI_PRED_FEATURES_REGISTRY",
    str(Path(__file__).resolve().parents[3] / "contracts" / "features" / "features.yaml"),
)

from ai_prediction_service import feature_flags
from ai_prediction_service.api import routes
from ai_prediction_service.db import engine
from ai_prediction_service.events import publisher
from ai_prediction_service.main import app
from ai_prediction_service.models import Base


async def _noop_publish(tenant_id, actor_user_id, predictions) -> None:
    pass


@pytest.fixture(autouse=True)
def suppress_event_publishing(monkeypatch: pytest.MonkeyPatch) -> None:
    """Keep API tests broker-free without replacing the implementation at import time."""
    monkeypatch.setattr(publisher, "publish_predictions", _noop_publish)


@pytest.fixture(autouse=True)
def enabled_test_tenant() -> None:
    original = feature_flags.gate.is_enabled("tenant-a", "ai_predictions")
    feature_flags.gate.set_enabled("tenant-a", "ai_predictions", True)
    yield
    feature_flags.gate.set_enabled("tenant-a", "ai_predictions", original)


@pytest.fixture(autouse=True)
def assigned_learner_scope(monkeypatch: pytest.MonkeyPatch) -> None:
    async def resolve(_tenant: str, _user: str, _role: str) -> set[str]:
        return {
            "11111111-1111-1111-1111-111111111111",
            "22222222-2222-2222-2222-222222222222",
            "33333333-3333-3333-3333-333333333333",
            "44444444-4444-4444-4444-444444444444",
            "55555555-5555-5555-5555-555555555555",
        }

    monkeypatch.setattr(routes, "resolve_learner_ids", resolve)


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


@pytest_asyncio.fixture
async def db_session() -> AsyncGenerator[AsyncSession]:
    async with AsyncSession(engine) as session:
        yield session
        await session.rollback()
