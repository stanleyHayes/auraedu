"""Production runtime configuration tests."""

import pytest
from ai_prediction_service.config import Settings, postgres_dsn, validate_production_runtime


def test_render_database_url_uses_asyncpg() -> None:
    configured = Settings(database_url="postgresql://user:secret@ai-db:5432/ai")
    assert configured.database_url == "postgresql+asyncpg://user:secret@ai-db:5432/ai"
    compose = Settings(database_url="postgres://user:secret@postgres:5432/ai?sslmode=disable")
    assert compose.database_url == "postgresql+asyncpg://user:secret@postgres:5432/ai?ssl=disable"
    assert postgres_dsn(compose.database_url) == (
        "postgresql://user:secret@postgres:5432/ai?sslmode=disable"
    )


def test_production_runtime_rejects_local_defaults(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("AI_PRED_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setenv("INTERNAL_SERVICE_TOKEN", "secret")
    with pytest.raises(RuntimeError, match="AI_PRED_DATABASE_URL"):
        validate_production_runtime(Settings())


def test_production_runtime_accepts_private_render_bindings(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("AI_PRED_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setenv("INTERNAL_SERVICE_TOKEN", "secret")
    configured = Settings(
        database_url="postgresql://user:secret@ai-db:5432/ai",
        nats_host="nats:4222",
        student_service_url="student-service:8090",
    )
    validate_production_runtime(configured)
