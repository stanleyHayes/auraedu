"""Tests for AI Prediction Service API."""

from datetime import UTC, datetime

from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "predictions:read,predictions:write",
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


async def test_ingest_and_list_metrics(client: AsyncClient):
    payload = _metric("11111111-1111-1111-1111-111111111111", "average_score", 55.0)
    response = await client.post(
        "/feature-store/metrics",
        json=payload,
        headers={**HEADERS, "X-Tenant-Id": "tenant-a"},
    )
    assert response.status_code == 201
    body = response.json()
    assert body["student_id"] == "11111111-1111-1111-1111-111111111111"
    assert body["metric_key"] == "average_score"


async def test_generate_predictions(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    for value in [55.0, 50.0]:
        await client.post(
            "/feature-store/metrics",
            json=_metric("11111111-1111-1111-1111-111111111111", "average_score", value),
            headers=tenant_headers,
        )
    await client.post(
        "/feature-store/metrics",
        json={
            "student_id": "11111111-1111-1111-1111-111111111111",
            "metric_key": "attendance_rate",
            "value": 0.6,
            "source": "attendance",
            "recorded_at": datetime.now(UTC).isoformat(),
        },
        headers=tenant_headers,
    )

    response = await client.post(
        "/predictions",
        json={
            "student_id": "11111111-1111-1111-1111-111111111111",
            "prediction_types": ["at_risk", "exam_readiness"],
        },
        headers=tenant_headers,
    )
    assert response.status_code == 201
    data = response.json()["data"]
    assert len(data) >= 2
    types = {item["prediction_type"] for item in data}
    assert "at_risk" in types
    assert "exam_readiness" in types


async def test_get_prediction(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json=_metric("22222222-2222-2222-2222-222222222222", "average_score", 45.0),
        headers=tenant_headers,
    )
    create_response = await client.post(
        "/predictions",
        json={"student_id": "22222222-2222-2222-2222-222222222222"},
        headers=tenant_headers,
    )
    prediction_id = create_response.json()["data"][0]["id"]

    response = await client.get(f"/predictions/{prediction_id}", headers=tenant_headers)
    assert response.status_code == 200
    assert response.json()["id"] == prediction_id


async def test_explain_prediction(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json=_metric("33333333-3333-3333-3333-333333333333", "average_score", 70.0),
        headers=tenant_headers,
    )
    create_response = await client.post(
        "/predictions",
        json={"student_id": "33333333-3333-3333-3333-333333333333"},
        headers=tenant_headers,
    )
    prediction_id = create_response.json()["data"][0]["id"]

    response = await client.get(
        f"/predictions/{prediction_id}/explain",
        headers=tenant_headers,
    )
    assert response.status_code == 200
    body = response.json()
    assert body["prediction_id"] == prediction_id
    assert len(body["factors"]) >= 1


async def test_approve_and_reject_prediction(client: AsyncClient):
    tenant_headers = {**HEADERS, "X-Tenant-Id": "tenant-a"}
    await client.post(
        "/feature-store/metrics",
        json=_metric("44444444-4444-4444-4444-444444444444", "average_score", 40.0),
        headers=tenant_headers,
    )
    create_response = await client.post(
        "/predictions",
        json={"student_id": "44444444-4444-4444-4444-444444444444"},
        headers=tenant_headers,
    )
    prediction_id = create_response.json()["data"][0]["id"]

    approve_response = await client.post(
        f"/predictions/{prediction_id}/approve",
        headers=tenant_headers,
    )
    assert approve_response.status_code == 200
    assert approve_response.json()["status"] == "approved"

    reject_response = await client.post(
        f"/predictions/{prediction_id}/reject",
        json={"reason": "Model over-estimated risk"},
        headers=tenant_headers,
    )
    assert reject_response.status_code == 200
    assert reject_response.json()["status"] == "rejected"
