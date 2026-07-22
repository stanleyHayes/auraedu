import asyncio
import logging
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any, cast

from sqlalchemy import select, text

from career_guidance_service.config import settings
from career_guidance_service.db import AsyncSessionLocal
from career_guidance_service.events.envelope import encode_event
from career_guidance_service.events.transport import ConsoleTransport, get_transport
from career_guidance_service.models import GuidanceOutbox

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient

logger = logging.getLogger(__name__)


def _require_nats(transport: object) -> None:
    if isinstance(transport, ConsoleTransport):
        raise ConnectionError


def guidance_outbox_event(
    item: GuidanceOutbox,
    *,
    occurred_at: datetime | None = None,
) -> dict[str, Any]:
    """Build the serialized contract envelope used by durable publication."""
    return {
        "specversion": "1.0",
        "type": item.event_type,
        "source": settings.service_name,
        "id": item.id,
        "time": (occurred_at or datetime.now(UTC)).isoformat(),
        "tenant_id": item.tenant_id,
        "datacontenttype": "application/json",
        "subject": f"students/{item.payload['student_id']}",
        "data": item.payload,
    }


class OutboxDispatcher:
    def __init__(self) -> None:
        self._task: asyncio.Task[None] | None = None

    async def start(self) -> None:
        self._task = asyncio.create_task(self._run())

    async def close(self) -> None:
        if self._task:
            self._task.cancel()
            await asyncio.gather(self._task, return_exceptions=True)
            self._task = None

    async def _run(self) -> None:
        while True:
            try:
                await self.dispatch_once()
            except Exception as exc:
                logger.warning("guidance outbox dispatch failed: %s", exc)
            await asyncio.sleep(1)

    async def dispatch_once(self) -> None:
        async with AsyncSessionLocal() as session:
            await session.execute(text("SELECT set_config('app.is_platform_admin','true',true)"))
            result = await session.execute(
                select(GuidanceOutbox)
                .where(
                    GuidanceOutbox.published_at.is_(None),
                    GuidanceOutbox.next_attempt_at <= datetime.now(UTC),
                )
                .order_by(GuidanceOutbox.created_at)
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
            error = None
            try:
                candidate = await get_transport()
                _require_nats(candidate)
                transport = cast("NATSClient", candidate)
                await transport.publish(
                    f"AURA.{item.event_type}",
                    encode_event(guidance_outbox_event(item)),
                    headers={"Nats-Msg-Id": item.id},
                )
            except Exception as exc:
                error = str(exc)[:1000]
            async with AsyncSessionLocal() as session:
                await session.execute(
                    text("SELECT set_config('app.is_platform_admin','true',true)")
                )
                persisted = await session.get(GuidanceOutbox, item.id)
                if persisted:
                    if error is None:
                        persisted.published_at = datetime.now(UTC)
                        persisted.last_error = None
                    else:
                        persisted.last_error = error
                await session.commit()
