"""Tests for Career Guidance Service feature-flag gating."""

import career_guidance_service.feature_flags as feature_flags
from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "guidance:read,guidance:write",
}


async def test_feature_disabled_blocks_guidance(client: AsyncClient):
    original = feature_flags.gate.is_enabled("tenant-a", "career_guidance")
    feature_flags.gate.set_enabled("tenant-a", "career_guidance", False)
    try:
        response = await client.get(
            "/guidance?student_id=11111111-1111-1111-1111-111111111111",
            headers={**HEADERS, "X-Tenant-Id": "tenant-a"},
        )
        assert response.status_code == 403
        assert response.json()["code"] == "feature_disabled"
    finally:
        feature_flags.gate.set_enabled("tenant-a", "career_guidance", original)


async def test_known_tenant_with_feature_off(client: AsyncClient):
    response = await client.get(
        "/guidance?student_id=11111111-1111-1111-1111-111111111111",
        headers={**HEADERS, "X-Tenant-Id": "aboom"},
    )
    assert response.status_code == 403
    assert response.json()["code"] == "feature_disabled"
