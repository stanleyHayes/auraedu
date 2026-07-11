"""Tests for Career Guidance Service API."""

from datetime import UTC, datetime

from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "guidance:read,guidance:write",
}


def _metric(student_id: str, metric_key: str, value: float) -> dict:
    return {
        "student_id": student_id,
        "metric_key": metric_key,
        "value": value,
        "source": "assessment",
        "recorded_at": datetime.now(UTC).isoformat(),
    }


async def test_health_check(client: AsyncClient):
    response = await client.get("/health")
    assert response.status_code == 200
    assert response.json()["status"] == "ok"


async def test_ingest_metric(client: AsyncClient):
    payload = _metric("11111111-1111-1111-1111-111111111111", "average_score", 85.0)
    response = await client.post(
        "/feature-store/metrics",
        json=payload,
        headers={**HEADERS, "X-Tenant-Id": "tenant-a"},
    )
    assert response.status_code == 201
    body = response.json()
    assert body["student_id"] == "11111111-1111-1111-1111-111111111111"
    assert body["metric_key"] == "average_score"


async def test_generate_guidance(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json=_metric("22222222-2222-2222-2222-222222222222", "average_score", 85.0),
        headers=tenant_headers,
    )
    await client.post(
        "/feature-store/metrics",
        json={
            "student_id": "22222222-2222-2222-2222-222222222222",
            "metric_key": "attendance_rate",
            "value": 0.92,
            "source": "attendance",
            "recorded_at": datetime.now(UTC).isoformat(),
        },
        headers=tenant_headers,
    )

    response = await client.post(
        "/guidance",
        json={
            "student_id": "22222222-2222-2222-2222-222222222222",
            "guidance_types": ["career_track", "course_load"],
        },
        headers=tenant_headers,
    )
    assert response.status_code == 201
    data = response.json()["data"]
    assert len(data) >= 2
    types = {item["guidance_type"] for item in data}
    assert "career_track" in types
    assert "course_load" in types


async def test_approve_and_reject_guidance(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json=_metric("33333333-3333-3333-3333-333333333333", "average_score", 70.0),
        headers=tenant_headers,
    )
    create_response = await client.post(
        "/guidance",
        json={"student_id": "33333333-3333-3333-3333-333333333333"},
        headers=tenant_headers,
    )
    guidance_id = create_response.json()["data"][0]["id"]

    approve_response = await client.post(
        f"/guidance/{guidance_id}/approve",
        headers=tenant_headers,
    )
    assert approve_response.status_code == 200
    assert approve_response.json()["status"] == "approved"

    reject_response = await client.post(
        f"/guidance/{guidance_id}/reject",
        json={"reason": "Not aligned with student interests"},
        headers=tenant_headers,
    )
    assert reject_response.status_code == 200
    assert reject_response.json()["status"] == "rejected"
