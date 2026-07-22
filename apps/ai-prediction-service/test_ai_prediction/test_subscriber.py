"""Tests for the AI Prediction feature-store event subscriber."""

import json
from types import SimpleNamespace
from typing import Any

from ai_prediction_service.events import subscriber as subscriber_module
from ai_prediction_service.events.subscriber import (
    MAX_DELIVERIES,
    MAX_EVENT_BYTES,
    STREAM_MAX_AGE_SECONDS,
    STREAM_MAX_MESSAGES,
    SUBSCRIBED_EVENTS,
    FeatureStoreSubscriber,
    process_event,
)
from ai_prediction_service.models import FeatureStoreMetric
from nats.js.api import AckPolicy, ConsumerConfig, RetentionPolicy, StreamConfig
from nats.js.errors import NotFoundError
from sqlalchemy import select


class RecordingJetStream:
    def __init__(self, existing: ConsumerConfig | None = None) -> None:
        self.options: list[dict[str, Any]] = []
        self.existing = existing
        self.updated: list[ConsumerConfig] = []

    async def consumer_info(self, _stream: str, _durable: str) -> Any:
        if self.existing is None:
            raise NotFoundError
        return SimpleNamespace(config=self.existing)

    async def add_consumer(self, _stream: str, *, config: ConsumerConfig) -> object:
        self.updated.append(config)
        return object()

    async def subscribe(self, _subject: str, **options: Any) -> object:
        self.options.append(options)
        return object()


class Message:
    def __init__(self, data: bytes) -> None:
        self.data = data
        self.acked = self.nacked = self.termed = 0

    async def ack(self) -> None:
        self.acked += 1

    async def nak(self) -> None:
        self.nacked += 1

    async def term(self) -> None:
        self.termed += 1


class ExistingStreamJetStream:
    def __init__(self) -> None:
        self.config = StreamConfig(name="AURA_EVENTS", subjects=["AURA.attendance.marked"])
        self.updated: StreamConfig | None = None

    async def stream_info(self, _name: str) -> Any:
        return SimpleNamespace(config=self.config)

    async def update_stream(self, *, config: StreamConfig) -> object:
        self.updated = config
        return object()


async def test_process_assessment_score_event(db_session):
    event = {
        "type": "assessment.score_recorded.v1",
        "tenant_id": "tenant-a",
        "data": {
            "student_id": "66666666-6666-6666-6666-666666666666",
            "score": 75,
            "max_score": 100,
        },
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.tenant_id == "tenant-a",
            FeatureStoreMetric.student_id == "66666666-6666-6666-6666-666666666666",
        )
    )
    metric = result.scalar_one()
    assert metric.metric_key == "assessment_score"
    assert metric.value == 0.75
    assert metric.source == "assessment"


async def test_process_attendance_event(db_session):
    event = {
        "type": "attendance.marked.v1",
        "tenant_id": "tenant-a",
        "data": {
            "student_id": "77777777-7777-7777-7777-777777777777",
            "status": "absent",
        },
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.tenant_id == "tenant-a",
            FeatureStoreMetric.student_id == "77777777-7777-7777-7777-777777777777",
        )
    )
    metric = result.scalar_one()
    assert metric.metric_key == "attendance_rate"
    assert metric.value == 0.0


async def test_unsupported_event_is_ignored(db_session):
    event = {
        "type": "payment.received",
        "tenant_id": "tenant-a",
        "data": {"student_id": "88888888-8888-8888-8888-888888888888"},
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.student_id == "88888888-8888-8888-8888-888888888888"
        )
    )
    assert result.scalar_one_or_none() is None


async def test_subscriber_uses_explicit_bounded_delivery() -> None:
    jetstream = RecordingJetStream()
    subscriber = FeatureStoreSubscriber()
    subscriber._js = jetstream  # type: ignore[assignment]
    await subscriber.start()

    assert len(jetstream.options) == len(SUBSCRIBED_EVENTS)
    assert all(option["manual_ack"] is True for option in jetstream.options)
    assert all(option["config"].ack_policy is AckPolicy.EXPLICIT for option in jetstream.options)
    assert all(option["config"].max_deliver == MAX_DELIVERIES for option in jetstream.options)


async def test_subscriber_reconciles_the_canonical_pubsub_stream() -> None:
    jetstream = ExistingStreamJetStream()
    subscriber = FeatureStoreSubscriber()
    subscriber._js = jetstream  # type: ignore[assignment]

    await subscriber._ensure_stream()

    assert jetstream.updated is not None
    assert jetstream.updated.subjects == ["AURA.>"]
    assert jetstream.updated.retention is RetentionPolicy.LIMITS
    assert jetstream.updated.max_msgs == STREAM_MAX_MESSAGES
    assert jetstream.updated.max_age == STREAM_MAX_AGE_SECONDS


async def test_subscriber_reconciles_existing_durable_without_recreating_it() -> None:
    existing = ConsumerConfig(
        durable_name="existing-durable",
        filter_subject="AURA.attendance.marked.v1",
        ack_policy=AckPolicy.NONE,
        max_deliver=-1,
    )
    jetstream = RecordingJetStream(existing)
    subscriber = FeatureStoreSubscriber()
    subscriber._js = jetstream  # type: ignore[assignment]

    await subscriber.start()

    assert len(jetstream.updated) == len(SUBSCRIBED_EVENTS)
    assert all(config.durable_name == "existing-durable" for config in jetstream.updated)
    assert all(config.filter_subject == "AURA.attendance.marked.v1" for config in jetstream.updated)
    assert all(config.ack_policy is AckPolicy.EXPLICIT for config in jetstream.updated)
    assert all(config.max_deliver == MAX_DELIVERIES for config in jetstream.updated)


async def test_subscriber_separates_poison_and_transient_failures(monkeypatch) -> None:
    subscriber = FeatureStoreSubscriber()
    malformed = Message(b"not-json")
    oversized = Message(b"x" * (MAX_EVENT_BYTES + 1))
    await subscriber._on_message(malformed)
    await subscriber._on_message(oversized)
    assert (malformed.termed, malformed.nacked, malformed.acked) == (1, 0, 0)
    assert (oversized.termed, oversized.nacked, oversized.acked) == (1, 0, 0)

    async def transient(_event: dict[str, Any]) -> None:
        raise RuntimeError

    monkeypatch.setattr(subscriber_module, "process_event", transient)
    retry = Message(json.dumps({"type": "attendance.marked.v1", "data": {}}).encode())
    await subscriber._on_message(retry)
    assert (retry.termed, retry.nacked, retry.acked) == (0, 1, 0)

    async def poison(_event: dict[str, Any]) -> None:
        raise ValueError

    monkeypatch.setattr(subscriber_module, "process_event", poison)
    invalid = Message(json.dumps({"type": "attendance.marked.v1", "data": {}}).encode())
    await subscriber._on_message(invalid)
    assert (invalid.termed, invalid.nacked, invalid.acked) == (1, 0, 0)
