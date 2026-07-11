"""Feature-flag gate loaded from the AuraEDU feature registry."""

import logging
import os
from pathlib import Path
from typing import Any

import yaml

logger = logging.getLogger(__name__)

DEFAULT_REGISTRY = "contracts/features/features.yaml"
FEATURE_KEY = "career_guidance"


class FeatureGate:
    """In-memory tenant feature-flag snapshot."""

    def __init__(self, tenant_features: dict[str, set[str]]) -> None:
        self._tenant_features = tenant_features

    def is_enabled(self, tenant_id: str, feature_key: str) -> bool:
        features = self._tenant_features.get(tenant_id)
        if features is None:
            # Unknown tenant: default to enabled so the gateway/tenant service remains
            # the authoritative rejection point for missing tenants.
            return True
        return feature_key in features

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
        defaults = feature.get("defaults", {})
        for tenant, state in defaults.items():
            if state in {"on", True}:
                tenant_features.setdefault(tenant, set()).add(key)
    return tenant_features


def _build_gate() -> FeatureGate:
    path = os.getenv("AI_PRED_FEATURES_REGISTRY", DEFAULT_REGISTRY)
    try:
        return FeatureGate(_load_registry(path))
    except Exception as exc:  # pragma: no cover - defensive fallback
        logger.warning(
            "Failed to load feature registry at %s; defaulting all features to enabled: %s",
            path,
            exc,
        )
        return FeatureGate({})


gate = _build_gate()


def is_enabled(tenant_id: str, feature_key: str = FEATURE_KEY) -> bool:
    return gate.is_enabled(tenant_id, feature_key)
