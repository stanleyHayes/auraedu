"""Test configuration for Career Guidance Service."""

import os
from collections.abc import AsyncGenerator
from pathlib import Path

import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession

os.environ.setdefault("AI_GUIDANCE_DATABASE_URL", "sqlite+aiosqlite:///:memory:")
os.environ.setdefault("AI_GUIDANCE_NATS_HOST", "")
os.environ.setdefault(
    "AI_GUIDANCE_FEATURES_REGISTRY",
    str(Path(__file__).resolve().parents[3] / "contracts" / "features" / "features.yaml"),
)

from career_guidance_service.db import engine
from career_guidance_service.events import publisher
from career_guidance_service.main import app
from career_guidance_service.models import Base


async def _noop_publish(tenant_id, actor_user_id, guidance_items) -> None:
    pass


publisher.publish_guidance = _noop_publish


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
