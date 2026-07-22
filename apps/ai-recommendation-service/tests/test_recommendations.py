"""Recommendation lifecycle tests."""

from datetime import UTC, datetime
from uuid import uuid4

import pytest
from ai_recommendation_service.api import routes
from httpx import AsyncClient


@pytest.mark.asyncio
async def test_permission_rejects_actor_tenant_header_confusion(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
) -> None:
    headers = {
        **actor_headers,
        "X-Tenant-Code": tenant_id,
        "X-Actor-Tenant": "other-school",
    }
    response = await client.post(
        "/feature-store/metrics",
        json={
            "student_id": str(uuid4()),
            "metric_key": "average_score",
            "value": 80,
            "source": "assessment",
            "recorded_at": datetime.now(UTC).isoformat(),
        },
        headers=headers,
    )

    assert response.status_code == 403
    assert response.json()["code"] == "tenant_mismatch"


@pytest.mark.asyncio
async def test_generate_and_approve_recommendation(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    student_id = str(uuid4())
    headers = {**actor_headers, "X-Tenant-Code": tenant_id, "X-Actor-Tenant": tenant_id}

    async def assigned_scope(_tenant: str, _user: str, _role: str) -> set[str]:
        return {student_id}

    monkeypatch.setattr(routes, "resolve_learner_ids", assigned_scope)

    # Seed a low average score metric.
    metric_payload = {
        "student_id": student_id,
        "metric_key": "average_score",
        "value": 45.0,
        "source": "assessment",
        "recorded_at": datetime.now(UTC).isoformat(),
    }
    response = await client.post("/feature-store/metrics", json=metric_payload, headers=headers)
    assert response.status_code == 201

    # Generate recommendations.
    response = await client.post(
        "/recommendations",
        json={"student_id": student_id},
        headers=headers,
    )
    assert response.status_code == 201
    data = response.json()["data"]
    assert any(r["recommendation_type"] == "academic_support" for r in data)
    rec = next(r for r in data if r["recommendation_type"] == "academic_support")
    assert rec["status"] == "pending"
    assert 0.0 <= rec["confidence"] <= 1.0

    # Approve the recommendation.
    response = await client.post(
        f"/recommendations/{rec['id']}/approve",
        json={},
        headers=headers,
    )
    assert response.status_code == 200
    assert response.json()["status"] == "approved"

    # Explain it.
    response = await client.get(f"/recommendations/{rec['id']}/explain", headers=headers)
    assert response.status_code == 200
    explanation = response.json()
    assert explanation["recommendation_id"] == rec["id"]
    assert any(f["metric_key"] == "average_score" for f in explanation["factors"])


@pytest.mark.asyncio
async def test_override_recommendation(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    student_id = str(uuid4())
    headers = {**actor_headers, "X-Tenant-Code": tenant_id, "X-Actor-Tenant": tenant_id}

    async def assigned_scope(_tenant: str, _user: str, _role: str) -> set[str]:
        return {student_id}

    monkeypatch.setattr(routes, "resolve_learner_ids", assigned_scope)

    response = await client.post(
        "/recommendations",
        json={"student_id": student_id},
        headers=headers,
    )
    assert response.status_code == 201
    rec = response.json()["data"][0]

    response = await client.post(
        f"/recommendations/{rec['id']}/override",
        json={"title": "Custom teacher plan", "note": "Follow up weekly"},
        headers=headers,
    )
    assert response.status_code == 200
    updated = response.json()
    assert updated["status"] == "overridden"
    assert updated["title"] == "Custom teacher plan"


@pytest.mark.asyncio
async def test_student_list_infers_owned_record_and_hides_other_students(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    student_id = str(uuid4())
    teacher_headers = {**actor_headers, "X-Tenant-Code": tenant_id, "X-Actor-Tenant": tenant_id}

    async def owned_scope(_tenant: str, _user: str, _role: str) -> set[str]:
        return {student_id}

    monkeypatch.setattr(routes, "resolve_learner_ids", owned_scope)
    response = await client.post(
        "/recommendations",
        json={"student_id": student_id},
        headers=teacher_headers,
    )
    assert response.status_code == 201
    recommendation_id = response.json()["data"][0]["id"]
    response = await client.post(
        f"/recommendations/{recommendation_id}/approve",
        json={},
        headers=teacher_headers,
    )
    assert response.status_code == 200

    student_headers = {
        "X-Tenant-Code": tenant_id,
        "X-Actor-Tenant": tenant_id,
        "X-Actor-User": str(uuid4()),
        "X-Actor-Role": "student",
        "X-Actor-Permissions": "ai.view_recommendations",
    }
    response = await client.get("/recommendations?status=approved", headers=student_headers)
    assert response.status_code == 200
    assert [item["id"] for item in response.json()["data"]] == [recommendation_id]
    gateway_response = await client.get(
        "/api/v1/ai/recommendations?status=approved",
        headers=student_headers,
    )
    assert gateway_response.status_code == 200
    assert [item["id"] for item in gateway_response.json()["data"]] == [recommendation_id]

    response = await client.get(
        f"/recommendations?student_id={uuid4()}&status=approved",
        headers=student_headers,
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_teacher_cannot_read_or_generate_for_unassigned_student(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    student_id = str(uuid4())
    admin_headers = {
        **actor_headers,
        "X-Tenant-Code": tenant_id,
        "X-Actor-Tenant": tenant_id,
        "X-Actor-Role": "admin",
    }
    response = await client.post(
        "/recommendations",
        json={"student_id": student_id},
        headers=admin_headers,
    )
    assert response.status_code == 201
    recommendation_id = response.json()["data"][0]["id"]

    async def no_assignments(_tenant: str, _user: str, _role: str) -> set[str]:
        return set()

    monkeypatch.setattr(routes, "resolve_learner_ids", no_assignments)
    teacher_headers = {
        **actor_headers,
        "X-Tenant-Code": tenant_id,
        "X-Actor-Tenant": tenant_id,
    }
    response = await client.post(
        "/recommendations",
        json={"student_id": student_id},
        headers=teacher_headers,
    )
    assert response.status_code == 404

    response = await client.get(
        f"/recommendations/{recommendation_id}",
        headers=teacher_headers,
    )
    assert response.status_code == 404
