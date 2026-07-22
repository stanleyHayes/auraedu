"""Defense-in-depth feature gate tests."""

import io
import json
from urllib.request import Request

import pytest
from ai_recommendation_service import feature_flags, learner_scope
from httpx import AsyncClient


async def test_disabled_tenant_is_rejected_by_service(
    client: AsyncClient,
    tenant_id: str,
    actor_headers: dict[str, str],
) -> None:
    feature_flags.gate.set_enabled(tenant_id, "ai_recommendations", False)
    headers = {**actor_headers, "X-Tenant-Code": tenant_id, "X-Actor-Tenant": tenant_id}
    response = await client.get("/recommendations", headers=headers)
    assert response.status_code == 403
    assert response.json()["code"] == "feature_disabled"


async def test_unknown_tenant_fails_closed(
    client: AsyncClient,
    actor_headers: dict[str, str],
) -> None:
    unknown = "44444444-4444-4444-8444-444444444444"
    headers = {**actor_headers, "X-Tenant-Code": unknown, "X-Actor-Tenant": unknown}
    response = await client.get("/recommendations", headers=headers)
    assert response.status_code == 403
    assert response.json()["code"] == "feature_disabled"


async def test_health_remains_ungated(client: AsyncClient) -> None:
    response = await client.get("/health")
    assert response.status_code == 200


def test_live_tenant_snapshot_overrides_static_default(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    tenant = "dynamic-school"
    payload = json.dumps(
        {
            "tenant_code": tenant,
            "features": [{"feature_key": "ai_recommendations", "is_enabled": True}],
        }
    ).encode()
    requested: list[str] = []
    monkeypatch.setenv("AI_REC_TENANT_SERVICE_URL", "tenant-service:8082")

    def open_snapshot(url: str, **_kwargs: object) -> io.BytesIO:
        requested.append(url)
        return io.BytesIO(payload)

    monkeypatch.setattr(feature_flags, "urlopen", open_snapshot)
    assert feature_flags.is_enabled(tenant, "ai_recommendations") is True
    assert requested == ["http://tenant-service:8082/api/v1/features?tenant=dynamic-school"]


def test_production_outage_never_uses_registry_default(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    tenant = "production-school"
    feature_flags.gate.set_enabled(tenant, "ai_recommendations", True)
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("AI_REC_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setattr(
        feature_flags,
        "urlopen",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(OSError("tenant service unavailable")),
    )
    assert feature_flags.is_enabled(tenant, "ai_recommendations") is False


def test_oversized_internal_responses_fail_closed(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    oversized = b"{}" + b" " * feature_flags.MAX_INTERNAL_JSON_RESPONSE_BYTES
    monkeypatch.setenv("AI_REC_TENANT_SERVICE_URL", "tenant-service:8082")
    monkeypatch.setattr(feature_flags, "urlopen", lambda *_args, **_kwargs: io.BytesIO(oversized))
    assert feature_flags._remote_enabled("dynamic-school", "ai_recommendations") is None

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
