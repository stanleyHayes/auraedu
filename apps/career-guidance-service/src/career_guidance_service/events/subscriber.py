"""Event subscriber that feeds the AI Guidance feature store."""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import Any

import nats
from nats.js import JetStreamContext

from career_guidance_service.config import settings
from career_guidance_service.db import AsyncSessionLocal
from career_guidance_service.models import FeatureStoreMetric

# Event types that produce feature-store metrics.
SUBSCRIBED_EVENTS = {
    "assessment.score_recorded.v1",
    "attendance.marked",
    "analytics.metric_updated",
}


def _metric_from_event(event_type: str, data: dict[str, Any]) -> FeatureStoreMetric | None:
    now = datetime.now(UTC)
    if event_type == "assessment.score_recorded.v1":
        max_score = data.get("max_score") or 0
        score = data.get("score", 0)
        value = (score / max_score) if max_score else score
        return FeatureStoreMetric(
            tenant_id="",  # filled from envelope
            student_id=data["student_id"],
            metric_key="assessment_score",
            value=value,
            source="assessment",
            recorded_at=now,
        )
    if event_type == "attendance.marked":
        status = data.get("status", "present")
        value = 1.0 if status in {"present", "excused"} else (0.5 if status == "late" else 0.0)
        return FeatureStoreMetric(
            tenant_id="",
            student_id=data["student_id"],
            metric_key="attendance_rate",
            value=value,
            source="attendance",
            recorded_at=now,
        )
    if event_type == "analytics.metric_updated":
        return FeatureStoreMetric(
            tenant_id="",
            student_id=data.get("student_id", ""),
            metric_key=data["metric_key"],
            value=float(data["value"]),
            source="analytics",
            recorded_at=now,
        )
    return None


async def process_event(event: dict[str, Any]) -> None:
    """Persist a single approved event as a feature-store metric."""
    event_type = event.get("type")
    if event_type not in SUBSCRIBED_EVENTS:
        return
    tenant_id = event.get("tenant_id")
    data = event.get("data", {})
    metric = _metric_from_event(event_type, data)
    if metric is None:
        return
    metric.tenant_id = tenant_id
    async with AsyncSessionLocal() as session:
        session.add(metric)
        await session.commit()


class FeatureStoreSubscriber:
    """Subscribes to upstream events and populates the feature store."""

    def __init__(self) -> None:
        self._nc: nats.NATS | None = None
        self._js: JetStreamContext | None = None
        self._subs: list[Any] = []

    async def connect(self) -> None:
        if not settings.nats_host:
            return
        nats_url = (
            settings.nats_host
            if settings.nats_host.startswith(("nats://", "tls://"))
            else f"nats://{settings.nats_host}"
        )
        self._nc = await nats.connect(nats_url)
        self._js = self._nc.jetstream()
        try:
            await self._js.add_stream(
                name="AURA_AI",
                subjects=[f"AURA.{event_type}" for event_type in SUBSCRIBED_EVENTS],
            )
        except nats.js.errors.BadRequestError:
            pass  # Stream may already exist with different config.

    async def start(self) -> None:
        if self._js is None:
            return
        for event_type in SUBSCRIBED_EVENTS:
            subject = f"AURA.{event_type}"
            sub = await self._js.subscribe(
                subject,
                stream="AURA_AI",
                durable=f"ai-pred-{event_type.replace('.', '-').replace('_', '-')}",
                cb=self._on_message,
            )
            self._subs.append(sub)

    async def _on_message(self, msg: Any) -> None:
        try:
            event = json.loads(msg.data.decode())
            await process_event(event)
            await msg.ack()
        except Exception:  # pragma: no cover - logged only
            await msg.nak()

    async def close(self) -> None:
        for sub in self._subs:
            await sub.unsubscribe()
        self._subs.clear()
        if self._nc:
            await self._nc.close()
            self._nc = None
            self._js = None
