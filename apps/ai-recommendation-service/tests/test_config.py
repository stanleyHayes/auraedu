"""Production transport configuration tests."""

import pytest
from ai_recommendation_service.config import (
    Settings,
    nats_url,
    postgres_dsn,
    validate_production_runtime,
)
from ai_recommendation_service.events import publisher as publisher_module
from ai_recommendation_service.events import subscriber as subscriber_module
from nats.js.errors import NotFoundError


@pytest.mark.parametrize(
    ("configured", "expected"),
    [
        ("nats.internal", "nats://nats.internal"),
        ("nats.internal:4222", "nats://nats.internal:4222"),
        ("nats://nats.internal:4222", "nats://nats.internal:4222"),
        ("tls://nats.internal:4222", "tls://nats.internal:4222"),
        ("  nats.internal  ", "nats://nats.internal"),
        ("", ""),
        ("   ", ""),
    ],
)
def test_nats_url_normalizes_render_private_hosts(configured: str, expected: str) -> None:
    assert nats_url(configured) == expected


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
    monkeypatch.setenv("AI_REC_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setenv("INTERNAL_SERVICE_TOKEN", "secret")
    with pytest.raises(RuntimeError, match="AI_REC_DATABASE_URL"):
        validate_production_runtime(Settings())


def test_production_runtime_accepts_private_render_bindings(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("AI_REC_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setenv("INTERNAL_SERVICE_TOKEN", "secret")
    configured = Settings(
        database_url="postgresql://user:secret@ai-db:5432/ai",
        nats_host="nats:4222",
        student_service_url="student-service:8090",
    )
    validate_production_runtime(configured)


class _FakeJetStream:
    async def stream_info(self, _name: str) -> None:
        raise NotFoundError

    async def add_stream(self, **_kwargs: object) -> None:
        return None


class _FakeNATSClient:
    def __init__(self) -> None:
        self._jetstream = _FakeJetStream()

    def jetstream(self) -> _FakeJetStream:
        return self._jetstream

    async def close(self) -> None:
        return None


@pytest.mark.asyncio
async def test_publisher_connect_normalizes_render_hostname(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    connected: list[str] = []

    async def connect(server: str) -> _FakeNATSClient:
        connected.append(server)
        return _FakeNATSClient()

    monkeypatch.setattr(publisher_module.settings, "nats_host", "nats.internal")
    monkeypatch.setattr(publisher_module.nats, "connect", connect)
    publisher = publisher_module.RecommendationPublisher()

    await publisher.connect()
    await publisher.close()

    assert connected == ["nats://nats.internal"]


@pytest.mark.asyncio
async def test_subscriber_connect_normalizes_render_hostname(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    connected: list[str] = []

    async def connect(server: str) -> _FakeNATSClient:
        connected.append(server)
        return _FakeNATSClient()

    monkeypatch.setattr(subscriber_module.settings, "nats_host", "nats.internal")
    monkeypatch.setattr(subscriber_module.nats, "connect", connect)
    subscriber = subscriber_module.FeatureStoreSubscriber()

    await subscriber.connect()
    await subscriber.close()

    assert connected == ["nats://nats.internal"]
