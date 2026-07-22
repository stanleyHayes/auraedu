"""Outbound CloudEvent envelope-limit regressions."""

import json
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any, cast

import pytest
from career_guidance_service.events import publisher
from career_guidance_service.events.envelope import (
    MAX_EVENT_BYTES,
    EventTooLargeError,
    encode_event,
)
from career_guidance_service.events.outbox import guidance_outbox_event
from career_guidance_service.models import GuidanceOutbox

from tools.testing import assert_event_contract

publish_guidance = publisher.publish_guidance
SCHEMA_PATH = Path(__file__).resolve().parents[3] / "contracts/events/ai.guidance_generated.v1.json"


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


def guidance_fixture() -> SimpleNamespace:
    return SimpleNamespace(
        student_id="11111111-1111-4111-8111-111111111111",
        guidance_type="career_track",
        confidence=0.9,
        explanation="Explore a science pathway.",
    )


def test_guidance_outbox_envelope_satisfies_contract() -> None:
    guidance = guidance_fixture()
    item = GuidanceOutbox(
        id="22222222-2222-4222-8222-222222222222",
        tenant_id="school-a",
        event_type="ai.guidance_generated.v1",
        payload=publisher.guidance_event_data(guidance),
    )
    event = guidance_outbox_event(
        item,
        occurred_at=datetime(2026, 7, 20, 10, 0, tzinfo=UTC),
    )
    assert_event_contract(SCHEMA_PATH, event)


async def test_guidance_publisher_emits_contract_data_and_rejects_oversize(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    transport = RecordingTransport()

    async def get_transport() -> Any:
        return cast(Any, transport)

    monkeypatch.setattr(publisher, "get_transport", get_transport)
    guidance = guidance_fixture()

    await publish_guidance("tenant-a", "teacher-1", [guidance])
    assert len(transport.calls) == 1
    event = json.loads(transport.calls[0][1])
    assert_event_contract(SCHEMA_PATH, event)
    assert event["data"] == publisher.guidance_event_data(guidance)

    guidance.explanation = "x" * MAX_EVENT_BYTES
    with pytest.raises(EventTooLargeError):
        await publish_guidance("tenant-a", "teacher-1", [guidance])
    assert len(transport.calls) == 1
