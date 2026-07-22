"""Fail-closed tenant feature snapshot for AI recommendations."""

import json
import logging
import os
from pathlib import Path
from typing import Any
from urllib.parse import quote
from urllib.request import urlopen

import yaml

logger = logging.getLogger(__name__)
DEFAULT_REGISTRY = "contracts/features/features.yaml"
FEATURE_KEY = "ai_recommendations"
MAX_INTERNAL_JSON_RESPONSE_BYTES = 1 << 20


class FeatureGate:
    def __init__(self, tenant_features: dict[str, set[str]]) -> None:
        self._tenant_features = tenant_features

    def is_enabled(self, tenant_id: str, feature_key: str) -> bool:
        features = self._tenant_features.get(tenant_id)
        return features is not None and feature_key in features

    def set_enabled(self, tenant_id: str, feature_key: str, enabled: bool) -> None:
        features = self._tenant_features.setdefault(tenant_id, set())
        if enabled:
            features.add(feature_key)
        else:
            features.discard(feature_key)


def _load_registry(path: str) -> dict[str, set[str]]:
    resolved = Path(path)
    if not resolved.is_absolute():
        resolved = Path.cwd() / resolved
    data: dict[str, Any] = yaml.safe_load(resolved.read_text())
    tenant_features: dict[str, set[str]] = {}
    for feature in data.get("features", []):
        key = feature["key"]
        for tenant, state in feature.get("defaults", {}).items():
            if state in {"on", True}:
                tenant_features.setdefault(tenant, set()).add(key)
    return tenant_features


def _build_gate() -> FeatureGate:
    path = os.getenv("AI_REC_FEATURES_REGISTRY", DEFAULT_REGISTRY)
    try:
        return FeatureGate(_load_registry(path))
    except Exception as exc:  # pragma: no cover
        logger.warning(
            "Failed to load feature registry at %s; defaulting all features to disabled: %s",
            path,
            exc,
        )
        return FeatureGate({})


gate = _build_gate()


def _decode_bounded_snapshot(response: Any) -> dict[str, Any]:
    raw_payload = response.read(MAX_INTERNAL_JSON_RESPONSE_BYTES + 1)
    if len(raw_payload) > MAX_INTERNAL_JSON_RESPONSE_BYTES:
        raise ValueError
    payload: dict[str, Any] = json.loads(raw_payload)
    return payload


def is_enabled(tenant_id: str, feature_key: str = FEATURE_KEY) -> bool:
    remote = _remote_enabled(tenant_id, feature_key)
    if remote is not None:
        return remote
    if os.getenv("ENVIRONMENT", "development").strip().lower() == "production":
        return False
    return gate.is_enabled(tenant_id, feature_key)


def _remote_enabled(tenant_id: str, feature_key: str) -> bool | None:
    base_url = os.getenv("AI_REC_TENANT_SERVICE_URL", "").rstrip("/")
    if not base_url:
        return None
    if "://" not in base_url:
        base_url = "http://" + base_url
    try:
        with urlopen(  # noqa: S310 - deployment-owned internal service URL
            f"{base_url}/api/v1/features?tenant={quote(tenant_id, safe='')}", timeout=1.0
        ) as response:
            payload = _decode_bounded_snapshot(response)
    except Exception as exc:
        logger.warning("Tenant feature lookup unavailable; using fail-closed snapshot: %s", exc)
        return None
    if payload.get("tenant_code") != tenant_id:
        return False
    return any(
        item.get("feature_key") == feature_key and item.get("is_enabled") is True
        for item in payload.get("features", [])
    )
