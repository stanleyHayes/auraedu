"""Outbound CloudEvent envelope-limit regressions."""

import json
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any, cast

import pytest
from ai_recommendation_service.events.envelope import (
    MAX_EVENT_BYTES,
    EventTooLargeError,
    encode_event,
)
from ai_recommendation_service.events.publisher import (
    RecommendationPublisher,
    recommendation_event_data,
    recommendation_outbox_event,
)
from ai_recommendation_service.models import RecommendationOutbox

from tools.testing import assert_event_contract

SCHEMA_PATH = (
    Path(__file__).resolve().parents[3] / "contracts/events/ai.recommendation_generated.v1.json"
)


class RecordingJetStream:
    def __init__(self) -> None:
        self.calls: list[tuple[str, bytes]] = []

    async def publish(self, subject: str, payload: bytes, **_kwargs: object) -> None:
        self.calls.append((subject, payload))


def test_encode_event_accepts_normal_envelope() -> None:
    payload = encode_event({"specversion": "1.0", "data": {"student_id": "student-1"}})
    assert len(payload) < MAX_EVENT_BYTES


def recommendation_fixture() -> SimpleNamespace:
    return SimpleNamespace(
        student_id="11111111-1111-4111-8111-111111111111",
        recommendation_type="academic_support",
        confidence=0.9,
        explanation="Review current learning plan.",
        status="pending",
    )


def test_recommendation_outbox_envelope_satisfies_contract() -> None:
    recommendation = recommendation_fixture()
    item = RecommendationOutbox(
        id="22222222-2222-4222-8222-222222222222",
        tenant_id="school-a",
        event_type="ai.recommendation_generated.v1",
        payload=recommendation_event_data(recommendation),
    )
    event = recommendation_outbox_event(
        item,
        occurred_at=datetime(2026, 7, 20, 10, 0, tzinfo=UTC),
    )
    assert_event_contract(SCHEMA_PATH, event)


async def test_recommendation_publisher_rejects_oversized_event_before_nats() -> None:
    broker = RecordingJetStream()
    publisher = RecommendationPublisher()
    publisher._js = cast(Any, broker)
    recommendation = recommendation_fixture()

    await publisher.publish_recommendation_generated("tenant-a", recommendation)
    assert len(broker.calls) == 1
    event = json.loads(broker.calls[0][1])
    assert_event_contract(SCHEMA_PATH, event)
    assert event["data"] == recommendation_event_data(recommendation)

    recommendation.explanation = "x" * MAX_EVENT_BYTES
    with pytest.raises(EventTooLargeError):
        await publisher.publish_recommendation_generated("tenant-a", recommendation)

    assert len(broker.calls) == 1
