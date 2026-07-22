"""Fresh-PostgreSQL proof for the shared AI database tenant boundary."""

# ruff: noqa: E501 - SQL fixtures remain readable as complete row/column declarations.

from __future__ import annotations

import os
from pathlib import Path
from uuid import UUID

import asyncpg
import pytest
from ai_recommendation_service.api import routes
from ai_recommendation_service.api.dependencies import Actor
from ai_recommendation_service.models import Recommendation
from sqlalchemy import text
from sqlalchemy.exc import SQLAlchemyError
from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine

POSTGRES_DSN = os.getenv("AI_RLS_TEST_DSN", "")
ROOT = Path(__file__).resolve().parents[3]
MIGRATIONS = (
    ROOT / "apps/ai-recommendation-service/migrations/0001_recommendations_rls.sql",
    ROOT / "apps/ai-recommendation-service/migrations/0002_transactional_outbox.sql",
    ROOT / "apps/ai-recommendation-service/migrations/0003_timezone_aware_timestamps.sql",
    ROOT / "apps/ai-prediction-service/migrations/0001_predictions_rls.sql",
    ROOT / "apps/ai-prediction-service/migrations/0002_transactional_outbox.sql",
    ROOT / "apps/ai-prediction-service/migrations/0003_timezone_aware_timestamps.sql",
    ROOT / "apps/career-guidance-service/migrations/0001_guidance_rls.sql",
    ROOT / "apps/career-guidance-service/migrations/0002_transactional_outbox.sql",
    ROOT / "apps/career-guidance-service/migrations/0003_timezone_aware_timestamps.sql",
)
TABLES = (
    "feature_store_metrics",
    "recommendations",
    "recommendation_outbox",
    "predictions",
    "prediction_outbox",
    "guidance",
    "guidance_outbox",
)


async def _set_runtime_tenant(connection: asyncpg.Connection, tenant_id: str) -> None:
    await connection.execute("SET LOCAL ROLE aura_ai_runtime")
    await connection.execute("SELECT set_config('app.tenant_id', $1, true)", tenant_id)


async def _drop_runtime_role(connection: asyncpg.Connection) -> None:
    exists = await connection.fetchval(
        "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'aura_ai_runtime')"
    )
    if exists:
        await connection.execute("DROP OWNED BY aura_ai_runtime")
        await connection.execute("DROP ROLE aura_ai_runtime")


async def _delete_probe_rows(connection: asyncpg.Connection) -> None:
    for table in TABLES:
        exists = await connection.fetchval("SELECT to_regclass($1) IS NOT NULL", table)
        if exists:
            await connection.execute(
                f"DELETE FROM {table} WHERE id LIKE 'rls-%'"  # noqa: S608 - fixed table allowlist
            )


@pytest.mark.skipif(not POSTGRES_DSN, reason="AI_RLS_TEST_DSN is required")
@pytest.mark.asyncio
async def test_shared_ai_schema_enforces_non_superuser_rls(  # noqa: PLR0915 - end-to-end DB proof
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    connection = await asyncpg.connect(POSTGRES_DSN)
    try:
        await _drop_runtime_role(connection)
        for migration in MIGRATIONS:
            await connection.execute(migration.read_text(encoding="utf-8"))

        await connection.execute("CREATE ROLE aura_ai_runtime NOLOGIN")
        await connection.execute(
            "GRANT SELECT, INSERT ON " + ", ".join(TABLES) + " TO aura_ai_runtime"
        )
        await connection.execute(
            """
            INSERT INTO feature_store_metrics
                (id, tenant_id, student_id, metric_key, value, source, recorded_at, created_at)
            VALUES
                ('rls-metric-upshs', 'upshs', 'student-upshs', 'attendance_rate', 0.9, 'test', now(), now()),
                ('rls-metric-aboom', 'aboom-ame-zion-c', 'student-aboom', 'attendance_rate', 0.8, 'test', now(), now());
            INSERT INTO recommendations
                (id, tenant_id, student_id, recommendation_type, title, status, confidence, created_at, updated_at)
            VALUES
                ('rls-rec-upshs', 'upshs', 'student-upshs', 'study', 'UPSHS', 'pending', 0.9, now(), now()),
                ('rls-rec-aboom', 'aboom-ame-zion-c', 'student-aboom', 'study', 'Aboom', 'pending', 0.8, now(), now());
            INSERT INTO predictions
                (id, tenant_id, student_id, prediction_type, title, value, confidence, status, explanation, created_at, updated_at)
            VALUES
                ('rls-pred-upshs', 'upshs', 'student-upshs', 'risk', 'UPSHS', 0.2, 0.9, 'pending', 'test', now(), now()),
                ('rls-pred-aboom', 'aboom-ame-zion-c', 'student-aboom', 'risk', 'Aboom', 0.3, 0.8, 'pending', 'test', now(), now());
            INSERT INTO guidance
                (id, tenant_id, student_id, guidance_type, title, value, confidence, status, explanation, created_at, updated_at)
            VALUES
                ('rls-guide-upshs', 'upshs', 'student-upshs', 'career', 'UPSHS', 0.7, 0.9, 'pending', 'test', now(), now()),
                ('rls-guide-aboom', 'aboom-ame-zion-c', 'student-aboom', 'career', 'Aboom', 0.6, 0.8, 'pending', 'test', now(), now());
            INSERT INTO recommendation_outbox (id, tenant_id, event_type, payload)
            VALUES
                ('rls-rec-outbox-upshs', 'upshs', 'ai.recommendation_generated.v1', '{"student_id":"student-upshs"}'),
                ('rls-rec-outbox-aboom', 'aboom-ame-zion-c', 'ai.recommendation_generated.v1', '{"student_id":"student-aboom"}');
            INSERT INTO prediction_outbox (id, tenant_id, event_type, payload)
            VALUES
                ('rls-pred-outbox-upshs', 'upshs', 'ai.prediction_generated.v1', '{"student_id":"student-upshs"}'),
                ('rls-pred-outbox-aboom', 'aboom-ame-zion-c', 'ai.prediction_generated.v1', '{"student_id":"student-aboom"}');
            INSERT INTO guidance_outbox (id, tenant_id, event_type, payload)
            VALUES
                ('rls-guide-outbox-upshs', 'upshs', 'ai.guidance_generated.v1', '{"student_id":"student-upshs"}'),
                ('rls-guide-outbox-aboom', 'aboom-ame-zion-c', 'ai.guidance_generated.v1', '{"student_id":"student-aboom"}');
            """
        )

        for table in TABLES:
            enabled, forced = await connection.fetchrow(
                "SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = $1::regclass",
                table,
            )
            assert enabled and forced

            transaction = connection.transaction()
            await transaction.start()
            try:
                await _set_runtime_tenant(connection, "upshs")
                count = await connection.fetchval(
                    f"SELECT count(*) FROM {table} WHERE id LIKE 'rls-%'"  # noqa: S608 - fixed table allowlist
                )
                assert count == 1
            finally:
                await transaction.rollback()

        denied_writes = (
            """INSERT INTO feature_store_metrics
                (id, tenant_id, student_id, metric_key, value, source, recorded_at, created_at)
                VALUES ('rls-metric-denied', 'aboom-ame-zion-c', 'student-x', 'score', 1, 'test', now(), now())""",
            """INSERT INTO recommendations
                (id, tenant_id, student_id, recommendation_type, title, status, confidence, created_at, updated_at)
                VALUES ('rls-rec-denied', 'aboom-ame-zion-c', 'student-x', 'study', 'Denied', 'pending', 1, now(), now())""",
            """INSERT INTO predictions
                (id, tenant_id, student_id, prediction_type, title, value, confidence, status, explanation, created_at, updated_at)
                VALUES ('rls-pred-denied', 'aboom-ame-zion-c', 'student-x', 'risk', 'Denied', 1, 1, 'pending', 'test', now(), now())""",
            """INSERT INTO guidance
                (id, tenant_id, student_id, guidance_type, title, value, confidence, status, explanation, created_at, updated_at)
                VALUES ('rls-guide-denied', 'aboom-ame-zion-c', 'student-x', 'career', 'Denied', 1, 1, 'pending', 'test', now(), now())""",
            """INSERT INTO recommendation_outbox (id, tenant_id, event_type, payload)
                VALUES ('rls-rec-outbox-denied', 'aboom-ame-zion-c', 'ai.recommendation_generated.v1', '{}')""",
            """INSERT INTO prediction_outbox (id, tenant_id, event_type, payload)
                VALUES ('rls-pred-outbox-denied', 'aboom-ame-zion-c', 'ai.prediction_generated.v1', '{}')""",
            """INSERT INTO guidance_outbox (id, tenant_id, event_type, payload)
                VALUES ('rls-guide-outbox-denied', 'aboom-ame-zion-c', 'ai.guidance_generated.v1', '{}')""",
        )
        for statement in denied_writes:
            transaction = connection.transaction()
            await transaction.start()
            try:
                await _set_runtime_tenant(connection, "upshs")
                with pytest.raises(asyncpg.InsufficientPrivilegeError):
                    await connection.execute(statement)
            finally:
                await transaction.rollback()

        # Prove the HTTP mutation and promised event share one transaction. A
        # forced outbox insert failure must leave no recommendation behind.
        await connection.execute(
            """
            CREATE OR REPLACE FUNCTION reject_atomic_recommendation_outbox()
            RETURNS trigger LANGUAGE plpgsql AS $$
            BEGIN
                IF NEW.payload->>'student_id' = '00000000-0000-0000-0000-000000000099' THEN
                    RAISE EXCEPTION 'forced outbox failure';
                END IF;
                RETURN NEW;
            END;
            $$;
            DROP TRIGGER IF EXISTS reject_atomic_recommendation_outbox
                ON recommendation_outbox;
            CREATE TRIGGER reject_atomic_recommendation_outbox
                BEFORE INSERT ON recommendation_outbox
                FOR EACH ROW EXECUTE FUNCTION reject_atomic_recommendation_outbox();
            """
        )

        student_id = "00000000-0000-0000-0000-000000000099"

        async def authorize_student(
            _tenant_id: str, _actor: Actor, requested_student_id: object
        ) -> object:
            return requested_student_id

        async def generate_one(
            _session: object,
            tenant_id: str,
            requested_student_id: object,
            _recommendation_types: object,
        ) -> list[Recommendation]:
            return [
                Recommendation(
                    id="rls-atomic-recommendation",
                    tenant_id=tenant_id,
                    student_id=str(requested_student_id),
                    recommendation_type="study",
                    title="Atomicity probe",
                    status="pending",
                    confidence=0.9,
                )
            ]

        monkeypatch.setattr(routes, "_authorized_student_id", authorize_student)
        monkeypatch.setattr(routes, "generate_recommendations", generate_one)
        sqlalchemy_dsn = POSTGRES_DSN.replace("postgresql://", "postgresql+asyncpg://", 1)
        test_engine = create_async_engine(sqlalchemy_dsn)
        test_sessions = async_sessionmaker(test_engine, expire_on_commit=False)
        try:
            async with test_sessions() as session:
                await session.execute(text("SELECT set_config('app.tenant_id','upshs',true)"))
                with pytest.raises(SQLAlchemyError, match="forced outbox failure"):
                    await routes.create_recommendations(
                        "upshs",
                        routes.GenerateRecommendationsRequest(student_id=UUID(student_id)),
                        session,
                        Actor(
                            user_id="11111111-1111-1111-1111-111111111111",
                            role="teacher",
                            tenant_id="upshs",
                            permissions="ai.approve_recommendations",
                        ),
                    )
                await session.rollback()
        finally:
            await test_engine.dispose()

        assert (
            await connection.fetchval(
                "SELECT count(*) FROM recommendations WHERE id = 'rls-atomic-recommendation'"
            )
            == 0
        )
    finally:
        await connection.execute(
            """
            DROP TRIGGER IF EXISTS reject_atomic_recommendation_outbox
                ON recommendation_outbox;
            DROP FUNCTION IF EXISTS reject_atomic_recommendation_outbox();
            """
        )
        await _delete_probe_rows(connection)
        await _drop_runtime_role(connection)
        await connection.close()
