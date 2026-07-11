"""Recommendation lifecycle tests."""

from datetime import UTC, datetime
from uuid import uuid4

import pytest
from httpx import AsyncClient


@pytest.mark.asyncio
async def test_generate_and_approve_recommendation(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
) -> None:
    student_id = str(uuid4())
    headers = {**actor_headers, "X-Tenant-Code": tenant_id}

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
) -> None:
    student_id = str(uuid4())
    headers = {**actor_headers, "X-Tenant-Code": tenant_id}

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
