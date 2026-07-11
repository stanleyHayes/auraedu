"""Event transport abstraction with NATS and fallback logger."""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING

import nats

from ai_prediction_service.config import settings

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient

logger = logging.getLogger(__name__)


class ConsoleTransport:
    """Fallback transport that logs events for local development."""

    async def publish(self, subject: str, payload: bytes) -> None:
        logger.info("EVENT subject=%s payload=%s", subject, payload.decode())


class _TransportManager:
    """Singleton manager for the NATS transport connection."""

    def __init__(self) -> None:
        self._client: NATSClient | None = None

    async def get(self, nats_host: str) -> ConsoleTransport | NATSClient:
        if self._client is not None:
            return self._client

        if not nats_host:
            return ConsoleTransport()

        nats_url = (
            nats_host if nats_host.startswith(("nats://", "tls://")) else f"nats://{nats_host}"
        )

        try:
            self._client = await nats.connect(nats_url)
        except Exception as exc:  # pragma: no cover
            logger.warning("NATS unavailable, using console transport: %s", exc)
            return ConsoleTransport()
        return self._client

    async def close(self) -> None:
        if self._client is not None:
            await self._client.close()
            self._client = None


_manager = _TransportManager()


async def get_transport() -> ConsoleTransport | NATSClient:
    return await _manager.get(settings.nats_host)


async def close_transport() -> None:
    await _manager.close()
