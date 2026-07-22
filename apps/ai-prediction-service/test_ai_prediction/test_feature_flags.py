"""Tests for AI Prediction Service feature-flag gating."""

import io
import json
from urllib.request import Request

import pytest
from ai_prediction_service import feature_flags, learner_scope
from httpx import AsyncClient

HEADERS = {
    "X-Actor-User": "teacher-1",
    "X-Actor-Role": "teacher",
    "X-Actor-Tenant": "tenant-a",
    "X-Actor-Permissions": "ai.view_predictions,ai.approve_predictions",
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


async def test_unknown_tenant_fails_closed(client: AsyncClient):
    response = await client.get(
        "/predictions?student_id=11111111-1111-1111-1111-111111111111",
        headers={**HEADERS, "X-Actor-Tenant": "unknown-school", "X-Tenant-Id": "unknown-school"},
    )
    assert response.status_code == 403
    assert response.json()["code"] == "feature_disabled"


def test_live_tenant_snapshot_overrides_static_default(monkeypatch):
    payload = json.dumps(
        {
            "tenant_code": "dynamic-school",
            "features": [{"feature_key": "ai_predictions", "is_enabled": True}],
        }
    ).encode()
    requested = []
    monkeypatch.setenv("AI_PRED_TENANT_SERVICE_URL", "tenant-service:8082")

    def open_snapshot(url, **_kwargs):
        requested.append(url)
        return io.BytesIO(payload)

    monkeypatch.setattr(feature_flags, "urlopen", open_snapshot)
    assert feature_flags.is_enabled("dynamic-school", "ai_predictions") is True
    assert requested == ["http://tenant-service:8082/api/v1/features?tenant=dynamic-school"]


def test_production_outage_never_uses_registry_default(monkeypatch):
    tenant = "production-school"
    feature_flags.gate.set_enabled(tenant, "ai_predictions", True)
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("AI_PRED_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setattr(
        feature_flags,
        "urlopen",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(OSError("tenant service unavailable")),
    )
    assert feature_flags.is_enabled(tenant, "ai_predictions") is False


def test_oversized_internal_responses_fail_closed(monkeypatch):
    oversized = b"{}" + b" " * feature_flags.MAX_INTERNAL_JSON_RESPONSE_BYTES
    monkeypatch.setenv("AI_PRED_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setattr(feature_flags, "urlopen", lambda *_args, **_kwargs: io.BytesIO(oversized))
    assert feature_flags._remote_enabled("dynamic-school", "ai_predictions") is None

    class ScopeResponse(io.BytesIO):
        status = 200

    monkeypatch.setattr(
        learner_scope,
        "urlopen",
        lambda *_args, **_kwargs: ScopeResponse(oversized),
    )
    with pytest.raises(learner_scope.LearnerScopeUnavailableError):
        learner_scope._read_scope_response(
            Request("http://student-service/internal/v1/learner-scope")
        )
