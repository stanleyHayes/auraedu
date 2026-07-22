"""Outbound CloudEvent envelope-limit regressions."""

import json
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any, cast

import pytest
from ai_prediction_service.events import publisher
from ai_prediction_service.events.envelope import (
    MAX_EVENT_BYTES,
    EventTooLargeError,
    encode_event,
)
from ai_prediction_service.events.outbox import prediction_outbox_event
from ai_prediction_service.models import PredictionOutbox

from tools.testing import assert_event_contract

publish_predictions = publisher.publish_predictions
SCHEMA_PATH = (
    Path(__file__).resolve().parents[3] / "contracts/events/ai.prediction_generated.v1.json"
)


class RecordingTransport:
    def __init__(self) -> None:
        self.calls: list[tuple[str, bytes]] = []

    async def publish(self, subject: str, payload: bytes, **_kwargs: object) -> None:
        self.calls.append((subject, payload))


def test_encode_event_accepts_normal_envelope() -> None:
    payload = encode_event({"specversion": "1.0", "data": {"student_id": "student-1"}})
    assert len(payload) < MAX_EVENT_BYTES


def test_encode_event_rejects_oversized_envelope() -> None:
    with pytest.raises(EventTooLargeError):
        encode_event({"data": {"explanation": "x" * MAX_EVENT_BYTES}})


def prediction_fixture() -> SimpleNamespace:
    return SimpleNamespace(
        student_id="11111111-1111-4111-8111-111111111111",
        prediction_type="dropout_risk",
        value=0.42,
        confidence=0.9,
        explanation="Current attendance and assessment trend.",
    )


def test_prediction_outbox_envelope_satisfies_contract() -> None:
    prediction = prediction_fixture()
    item = PredictionOutbox(
        id="22222222-2222-4222-8222-222222222222",
        tenant_id="school-a",
        event_type="ai.prediction_generated.v1",
        payload=publisher.prediction_event_data(prediction),
    )
    event = prediction_outbox_event(
        item,
        occurred_at=datetime(2026, 7, 20, 10, 0, tzinfo=UTC),
    )
    assert_event_contract(SCHEMA_PATH, event)


async def test_prediction_publisher_emits_contract_data_and_rejects_oversize(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    transport = RecordingTransport()

    async def get_transport() -> Any:
        return cast(Any, transport)

    monkeypatch.setattr(publisher, "get_transport", get_transport)
    prediction = prediction_fixture()

    await publish_predictions("tenant-a", "teacher-1", [prediction])
    assert len(transport.calls) == 1
    subject, payload = transport.calls[0]
    event = json.loads(payload)
    assert_event_contract(SCHEMA_PATH, event)
    assert subject == "ai.prediction_generated.v1"
    assert event["data"] == publisher.prediction_event_data(prediction)
    assert event["data"]["value"] == 0.42

    prediction.explanation = "x" * MAX_EVENT_BYTES
    with pytest.raises(EventTooLargeError):
        await publish_predictions("tenant-a", "teacher-1", [prediction])
    assert len(transport.calls) == 1
