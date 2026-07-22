"""CloudEvent publisher for AI recommendation events."""

from __future__ import annotations

import asyncio
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any

import nats
from nats.js import JetStreamContext
from sqlalchemy import select, text

from ai_recommendation_service.config import nats_url, settings
from ai_recommendation_service.db import AsyncSessionLocal
from ai_recommendation_service.events.envelope import encode_event
from ai_recommendation_service.models import RecommendationOutbox

__all__ = ["RecommendationPublisher", "nats", "settings"]

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient


@dataclass
class CloudEvent:
    """Minimal CloudEvents 1.0 envelope."""

    specversion: str
    type: str
    source: str
    id: str
    time: str
    tenant_id: str
    subject: str
    data: dict[str, Any]

    def to_dict(self) -> dict[str, Any]:
        return {
            "specversion": self.specversion,
            "type": self.type,
            "source": self.source,
            "id": self.id,
            "time": self.time,
            "tenant_id": self.tenant_id,
            "subject": self.subject,
            "datacontenttype": "application/json",
            "data": self.data,
        }


def recommendation_event_data(recommendation: Any) -> dict[str, Any]:
    """Build the contract-owned data object for direct and outbox publication."""
    return {
        "student_id": recommendation.student_id,
        "recommendation_type": recommendation.recommendation_type,
        "confidence": recommendation.confidence,
        "explanation": recommendation.explanation,
        "status": recommendation.status,
    }


def recommendation_outbox_event(
    item: RecommendationOutbox,
    *,
    occurred_at: datetime | None = None,
) -> dict[str, Any]:
    """Build the serialized contract envelope used by durable publication."""
    return CloudEvent(
        specversion="1.0",
        type=item.event_type,
        source=settings.service_name,
        id=item.id,
        time=(occurred_at or datetime.now(UTC)).isoformat(),
        tenant_id=item.tenant_id,
        subject=str(item.payload.get("student_id", "")),
        data=item.payload,
    ).to_dict()


class RecommendationPublisher:
    """Publishes `ai.recommendation_generated.v1` events to NATS JetStream."""

    def __init__(self) -> None:
        self._nc: NATSClient | None = None
        self._js: JetStreamContext | None = None
        self._task: asyncio.Task[None] | None = None

    async def connect(self) -> None:
        server = nats_url(settings.nats_host)
        if not server:
            return
        try:
            self._nc = await nats.connect(server)
            self._js = self._nc.jetstream()
        except Exception as exc:  # pragma: no cover - connectivity failure logged only
            print(f"publisher: NATS connection failed: {exc}")
        if self._task is None:
            self._task = asyncio.create_task(self._dispatch_loop())

    async def close(self) -> None:
        if self._task:
            self._task.cancel()
            await asyncio.gather(self._task, return_exceptions=True)
            self._task = None
        if self._nc:
            await self._nc.close()
            self._nc = None
            self._js = None

    async def _dispatch_loop(self) -> None:
        while True:
            try:
                await self._dispatch_once()
            except Exception as exc:  # pragma: no cover - retried background dependency
                print(f"publisher: outbox dispatch failed: {exc}")
            await asyncio.sleep(1)

    async def _dispatch_once(self) -> None:
        if self._js is None:
            server = nats_url(settings.nats_host)
            if not server:
                return
            self._nc = await nats.connect(server)
            self._js = self._nc.jetstream()
        async with AsyncSessionLocal() as session:
            await session.execute(text("SELECT set_config('app.is_platform_admin','true',true)"))
            result = await session.execute(
                select(RecommendationOutbox)
                .where(
                    RecommendationOutbox.published_at.is_(None),
                    RecommendationOutbox.next_attempt_at <= datetime.now(UTC),
                )
                .order_by(RecommendationOutbox.created_at)
                .with_for_update(skip_locked=True)
                .limit(25)
            )
            items = list(result.scalars())
            for item in items:
                item.attempts += 1
                item.next_attempt_at = datetime.now(UTC) + timedelta(
                    seconds=min(300, 2 ** min(item.attempts, 8))
                )
            await session.commit()
        for item in items:
            try:
                await self._js.publish(
                    f"AURA.{item.event_type}",
                    encode_event(recommendation_outbox_event(item)),
                    headers={"Nats-Msg-Id": item.id},
                )
                error = None
            except Exception as exc:  # pragma: no cover - broker failure retry path
                error = str(exc)[:1000]
            async with AsyncSessionLocal() as session:
                await session.execute(
                    text("SELECT set_config('app.is_platform_admin','true',true)")
                )
                persisted = await session.get(RecommendationOutbox, item.id)
                if persisted:
                    if error is None:
                        persisted.published_at = datetime.now(UTC)
                        persisted.last_error = None
                    else:
                        persisted.last_error = error
                await session.commit()

    async def publish_recommendation_generated(
        self,
        tenant_id: str,
        recommendation: Any,  # Recommendation ORM
    ) -> None:
        if self._js is None:
            return
        event = CloudEvent(
            specversion="1.0",
            type="ai.recommendation_generated.v1",
            source=settings.service_name,
            id=str(uuid.uuid4()),
            time=datetime.now(UTC).isoformat(),
            tenant_id=tenant_id,
            subject=recommendation.student_id,
            data=recommendation_event_data(recommendation),
        )
        await self._js.publish(
            f"AURA.{event.type}",
            encode_event(event.to_dict()),
            headers={"Nats-Msg-Id": event.id},
        )


class RecordingPublisher(RecommendationPublisher):
    """Records published events for tests."""

    def __init__(self) -> None:
        super().__init__()
        self.events: list[CloudEvent] = []

    async def connect(self) -> None:
        pass

    async def close(self) -> None:
        pass

    async def publish_recommendation_generated(
        self,
        tenant_id: str,
        recommendation: Any,
    ) -> None:
        self.events.append(
            CloudEvent(
                specversion="1.0",
                type="ai.recommendation_generated.v1",
                source=settings.service_name,
                id=str(uuid.uuid4()),
                time=datetime.now(UTC).isoformat(),
                tenant_id=tenant_id,
                subject=recommendation.student_id,
                data=recommendation_event_data(recommendation),
            )
        )
