"""Database engine, migrations, and tenant-scoped session management."""

from pathlib import Path
from typing import TYPE_CHECKING, cast

import asyncpg
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

if TYPE_CHECKING:
    from sqlalchemy import MetaData

from ai_prediction_service.config import postgres_dsn, settings

engine = create_async_engine(settings.database_url, echo=settings.debug, future=True)
AsyncSessionLocal = async_sessionmaker(bind=engine, expire_on_commit=False)

_MIGRATIONS = Path(__file__).resolve().parents[2] / "migrations"
_MIGRATION_LOCK = 8_203_001


async def initialize_database(metadata: MetaData) -> None:
    """Apply versioned PostgreSQL migrations; retain SQLite for unit tests."""
    if engine.dialect.name != "postgresql":
        async with engine.begin() as conn:
            await conn.run_sync(metadata.create_all)
        return

    dsn = postgres_dsn(settings.database_url)
    connection = cast("asyncpg.Connection", await asyncpg.connect(dsn))
    try:
        async with connection.transaction():
            await connection.execute("SELECT pg_advisory_xact_lock($1)", _MIGRATION_LOCK)
            await connection.execute(
                """
                CREATE TABLE IF NOT EXISTS aura_ai_schema_migrations (
                    service_name text NOT NULL,
                    version text NOT NULL,
                    applied_at timestamptz NOT NULL DEFAULT now(),
                    PRIMARY KEY (service_name, version)
                )
                """
            )
            for migration in sorted(_MIGRATIONS.glob("*.sql")):
                applied = cast(
                    "object | None",
                    await connection.fetchval(
                        "SELECT 1 FROM aura_ai_schema_migrations "
                        "WHERE service_name = $1 AND version = $2",
                        settings.service_name,
                        migration.name,
                    ),
                )
                if applied:
                    continue
                await connection.execute(migration.read_text(encoding="utf-8"))
                await connection.execute(
                    "INSERT INTO aura_ai_schema_migrations (service_name, version) VALUES ($1, $2)",
                    settings.service_name,
                    migration.name,
                )
    finally:
        await connection.close()


async def set_tenant_context(session: AsyncSession, tenant_id: str) -> None:
    """Bind the current transaction to one canonical tenant code for PostgreSQL RLS."""
    bind = session.get_bind()
    if bind.dialect.name == "postgresql":
        await session.execute(
            text("SELECT set_config('app.tenant_id', :tenant_id, true)"),
            {"tenant_id": tenant_id},
        )
