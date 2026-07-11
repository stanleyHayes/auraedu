"""Tests for AI Prediction Service feature-flag gating."""

import ai_prediction_service.feature_flags as feature_flags
from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "predictions:read,predictions:write",
}


async def test_feature_disabled_blocks_predictions(client: AsyncClient):
    original = feature_flags.gate.is_enabled("tenant-a", "ai_predictions")
    feature_flags.gate.set_enabled("tenant-a", "ai_predictions", False)
    try:
        response = await client.get(
            "/predictions?student_id=11111111-1111-1111-1111-111111111111",
            headers={**HEADERS, "X-Tenant-Id": "tenant-a"},
        )
        assert response.status_code == 403
        assert response.json()["code"] == "feature_disabled"
    finally:
        feature_flags.gate.set_enabled("tenant-a", "ai_predictions", original)


async def test_known_tenant_with_feature_off(client: AsyncClient):
    # aboom has ai_predictions off by default in contracts/features/features.yaml
    response = await client.get(
        "/predictions?student_id=11111111-1111-1111-1111-111111111111",
        headers={**HEADERS, "X-Tenant-Id": "aboom"},
    )
    assert response.status_code == 403
    assert response.json()["code"] == "feature_disabled"
