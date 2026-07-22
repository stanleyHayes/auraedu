"""Tests for Career Guidance Service API."""

from datetime import UTC, datetime

import pytest
from career_guidance_service.api import routes
from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "ai.view_guidance,ai.approve_guidance",
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


async def test_readiness_checks_database(client: AsyncClient):
    response = await client.get("/ready")
    assert response.status_code == 200
    assert response.json()["status"] == "ready"


async def test_readiness_fails_closed_without_database(
    client: AsyncClient, monkeypatch: pytest.MonkeyPatch
):
    async def unavailable() -> bool:
        return False

    monkeypatch.setattr(routes, "database_ready", unavailable)
    response = await client.get("/ready")
    assert response.status_code == 503
    assert response.json()["code"] == "not_ready"


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


async def test_permission_rejects_actor_tenant_header_confusion(client: AsyncClient):
    response = await client.post(
        "/feature-store/metrics",
        json=_metric("11111111-1111-1111-1111-111111111111", "average_score", 85.0),
        headers={**HEADERS, "X-Tenant-Id": "tenant-a", "X-Actor-Tenant": "other-school"},
    )

    assert response.status_code == 403
    assert response.json()["code"] == "tenant_mismatch"


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


async def test_student_sees_only_approved_owned_guidance(client: AsyncClient):
    student_id = "33333333-3333-3333-3333-333333333333"
    teacher_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    create_response = await client.post(
        "/guidance",
        json={"student_id": student_id},
        headers=teacher_headers,
    )
    guidance_id = create_response.json()["data"][0]["id"]
    student_headers = {
        "X-Tenant-Id": "tenant-a",
        "X-Actor-User": "student-user",
        "X-Actor-Role": "student",
        "X-Actor-Tenant": "tenant-a",
        "X-Actor-Permissions": "ai.view_guidance",
    }

    response = await client.get("/guidance", headers=student_headers)
    assert response.status_code == 200
    assert response.json()["data"] == []
    response = await client.get(f"/guidance/{guidance_id}", headers=student_headers)
    assert response.status_code == 404

    response = await client.post(
        f"/guidance/{guidance_id}/approve",
        headers=teacher_headers,
    )
    assert response.status_code == 200
    response = await client.get("/guidance", headers=student_headers)
    assert [item["id"] for item in response.json()["data"]] == [guidance_id]
    gateway_response = await client.get(
        "/api/v1/ai/career-guidance/guidance",
        headers=student_headers,
    )
    assert gateway_response.status_code == 200
    assert [item["id"] for item in gateway_response.json()["data"]] == [guidance_id]
