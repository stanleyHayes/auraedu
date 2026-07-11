"""Tests for AI Prediction Service event publishing."""

from datetime import UTC, datetime

import pytest
from ai_prediction_service.events import publisher
from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "predictions:read,predictions:write",
}


@pytest.fixture
def recording_publisher(monkeypatch):
    calls = []

    async def _publish(tenant_id, actor_user_id, predictions):
        calls.append(
            {
                "tenant_id": tenant_id,
                "actor_user_id": actor_user_id,
                "predictions": predictions,
            }
        )

    monkeypatch.setattr(publisher, "publish_predictions", _publish)
    yield calls


async def test_prediction_generates_cloud_event(
    client: AsyncClient,
    recording_publisher,
):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json={
            "student_id": "55555555-5555-5555-5555-555555555555",
            "metric_key": "average_score",
            "value": 45.0,
            "source": "assessment",
            "recorded_at": datetime.now(UTC).isoformat(),
        },
        headers=tenant_headers,
    )

    response = await client.post(
        "/predictions",
        json={"student_id": "55555555-5555-5555-5555-555555555555"},
        headers=tenant_headers,
    )
    assert response.status_code == 201
    assert len(recording_publisher) == 1
    event_batch = recording_publisher[0]
    assert event_batch["tenant_id"] == "tenant-a"
    assert event_batch["actor_user_id"] == "teacher-1"
    assert len(event_batch["predictions"]) >= 1
