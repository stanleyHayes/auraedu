"""Event subscriber that feeds the AI Prediction feature store."""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Any

import nats
from nats.js import JetStreamContext
from nats.js.api import AckPolicy, ConsumerConfig, RetentionPolicy

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient

from ai_prediction_service.config import settings
from ai_prediction_service.db import AsyncSessionLocal, set_tenant_context
from ai_prediction_service.events.envelope import MAX_EVENT_BYTES
from ai_prediction_service.models import FeatureStoreMetric

# Event types that produce feature-store metrics.
SUBSCRIBED_EVENTS = {
    "assessment.score_recorded.v1",
    "attendance.marked.v1",
    "analytics.metric_updated.v1",
}
DURABLE_PREFIX = "ai-pred"
MAX_DELIVERIES = 5
ACK_WAIT_SECONDS = 30.0
STREAM_NAME = "AURA_EVENTS"
STREAM_MAX_MESSAGES = 1_000_000
STREAM_MAX_AGE_SECONDS = 7 * 24 * 60 * 60


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
    if event_type == "attendance.marked.v1":
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
    if event_type == "analytics.metric_updated.v1":
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
    tenant_id: str | None = event.get("tenant_id")
    data = event.get("data", {})
    metric = _metric_from_event(event_type, data)
    if metric is None or tenant_id is None:
        return
    metric.tenant_id = tenant_id
    async with AsyncSessionLocal() as session:
        await set_tenant_context(session, tenant_id)
        session.add(metric)
        await session.commit()


class FeatureStoreSubscriber:
    """Subscribes to upstream events and populates the feature store."""

    def __init__(self) -> None:
        self._nc: NATSClient | None = None
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
        await self._ensure_stream()

    async def _ensure_stream(self) -> None:
        """Create or reconcile the canonical pub/sub stream without competing subject ownership."""
        jetstream = self._js
        if jetstream is None:
            return
        subjects = ["AURA.>"]
        try:
            info = await jetstream.stream_info(STREAM_NAME)
        except nats.js.errors.NotFoundError:
            await jetstream.add_stream(
                name=STREAM_NAME,
                subjects=subjects,
                retention=RetentionPolicy.LIMITS,
                max_msgs=STREAM_MAX_MESSAGES,
                max_age=STREAM_MAX_AGE_SECONDS,
            )
            return
        config = info.config
        if (
            sorted(config.subjects or []) != subjects
            or config.retention is not RetentionPolicy.LIMITS
            or config.max_msgs != STREAM_MAX_MESSAGES
            or config.max_age != STREAM_MAX_AGE_SECONDS
        ):
            await jetstream.update_stream(
                config=config.evolve(
                    subjects=subjects,
                    retention=RetentionPolicy.LIMITS,
                    max_msgs=STREAM_MAX_MESSAGES,
                    max_age=STREAM_MAX_AGE_SECONDS,
                )
            )

    async def start(self) -> None:
        if self._js is None:
            return
        for event_type in SUBSCRIBED_EVENTS:
            subject = f"AURA.{event_type}"
            durable = f"{DURABLE_PREFIX}-{event_type.replace('.', '-').replace('_', '-')}"
            await self._reconcile_consumer(durable)
            sub = await self._js.subscribe(
                subject,
                stream=STREAM_NAME,
                durable=durable,
                cb=self._on_message,
                manual_ack=True,
                config=ConsumerConfig(
                    ack_policy=AckPolicy.EXPLICIT,
                    ack_wait=ACK_WAIT_SECONDS,
                    max_deliver=MAX_DELIVERIES,
                ),
            )
            self._subs.append(sub)

    async def _reconcile_consumer(self, durable: str) -> None:
        """Apply the retry policy to an existing durable without resetting its state."""
        jetstream = self._js
        if jetstream is None:
            return
        try:
            info = await jetstream.consumer_info(STREAM_NAME, durable)
        except nats.js.errors.NotFoundError:
            return
        config = info.config
        if (
            config.ack_policy is AckPolicy.EXPLICIT
            and config.ack_wait == ACK_WAIT_SECONDS
            and config.max_deliver == MAX_DELIVERIES
        ):
            return
        await jetstream.add_consumer(
            STREAM_NAME,
            config=config.evolve(
                ack_policy=AckPolicy.EXPLICIT,
                ack_wait=ACK_WAIT_SECONDS,
                max_deliver=MAX_DELIVERIES,
            ),
        )

    async def _on_message(self, msg: Any) -> None:
        try:
            if len(msg.data) > MAX_EVENT_BYTES:
                await msg.term()
                return
            event = json.loads(msg.data.decode())
        except UnicodeDecodeError, json.JSONDecodeError:
            await msg.term()
            return
        try:
            await process_event(event)
        except KeyError, TypeError, ValueError:
            await msg.term()
        except Exception:  # pragma: no cover - transient dependency failure
            await msg.nak()
        else:
            await msg.ack()

    async def close(self) -> None:
        for sub in self._subs:
            await sub.unsubscribe()
        self._subs.clear()
        if self._nc:
            await self._nc.close()
            self._nc = None
            self._js = None
