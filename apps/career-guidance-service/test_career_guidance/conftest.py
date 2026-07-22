"""Test configuration for Career Guidance Service."""

import os
from collections.abc import AsyncGenerator
from pathlib import Path

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession

os.environ.setdefault("AI_GUIDANCE_DATABASE_URL", "sqlite+aiosqlite:///:memory:")
os.environ.setdefault("AI_GUIDANCE_NATS_HOST", "")
os.environ.setdefault(
    "AI_GUIDANCE_FEATURES_REGISTRY",
    str(Path(__file__).resolve().parents[3] / "contracts" / "features" / "features.yaml"),
)

from career_guidance_service import feature_flags
from career_guidance_service.api import routes
from career_guidance_service.db import engine
from career_guidance_service.events import publisher
from career_guidance_service.main import app
from career_guidance_service.models import Base


async def _noop_publish(tenant_id, actor_user_id, guidance_items) -> None:
    pass


@pytest.fixture(autouse=True)
def suppress_event_publishing(monkeypatch: pytest.MonkeyPatch) -> None:
    """Keep API tests broker-free without replacing the implementation at import time."""
    monkeypatch.setattr(publisher, "publish_guidance", _noop_publish)


@pytest.fixture(autouse=True)
def enabled_test_tenant() -> None:
    """Tests opt the synthetic tenant into the otherwise fail-closed feature."""
    original = feature_flags.gate.is_enabled("tenant-a", "career_guidance")
    feature_flags.gate.set_enabled("tenant-a", "career_guidance", True)
    yield
    feature_flags.gate.set_enabled("tenant-a", "career_guidance", original)


@pytest.fixture(autouse=True)
def assigned_learner_scope(monkeypatch: pytest.MonkeyPatch) -> None:
    async def resolve(_tenant: str, _user: str, _role: str) -> set[str]:
        if _role == "student":
            return {"33333333-3333-3333-3333-333333333333"}
        return {
            "11111111-1111-1111-1111-111111111111",
            "22222222-2222-2222-2222-222222222222",
            "33333333-3333-3333-3333-333333333333",
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
