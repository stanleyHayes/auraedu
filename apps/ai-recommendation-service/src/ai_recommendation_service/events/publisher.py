"""CloudEvent publisher for AI recommendation events."""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

import nats
from nats.js import JetStreamContext

from ai_recommendation_service.config import settings


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


class RecommendationPublisher:
    """Publishes `ai.recommendation_generated.v1` events to NATS JetStream."""

    def __init__(self) -> None:
        self._nc: nats.NATS | None = None
        self._js: JetStreamContext | None = None

    async def connect(self) -> None:
        if not settings.nats_host:
            return
        try:
            self._nc = await nats.connect(settings.nats_host)
            self._js = self._nc.jetstream()
        except Exception as exc:  # pragma: no cover - connectivity failure logged only
            print(f"publisher: NATS connection failed: {exc}")

    async def close(self) -> None:
        if self._nc:
            await self._nc.close()
            self._nc = None
            self._js = None

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
            data={
                "student_id": recommendation.student_id,
                "recommendation_type": recommendation.recommendation_type,
                "confidence": recommendation.confidence,
                "explanation": recommendation.explanation,
                "status": recommendation.status,
            },
        )
        await self._js.publish(
            f"AURA.{event.type}",
            json.dumps(event.to_dict()).encode(),
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
                data={
                    "student_id": recommendation.student_id,
                    "recommendation_type": recommendation.recommendation_type,
                    "confidence": recommendation.confidence,
                    "explanation": recommendation.explanation,
                    "status": recommendation.status,
                },
            )
        )
