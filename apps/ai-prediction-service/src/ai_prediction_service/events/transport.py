"""Event transport abstraction with NATS and fallback logger."""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from ai_prediction_service.config import settings

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient


logger = logging.getLogger(__name__)
_client: NATSClient | None = None


class ConsoleTransport:
    """Fallback transport that logs events for local development."""

    async def publish(self, subject: str, payload: bytes) -> None:
        logger.info("EVENT subject=%s payload=%s", subject, payload.decode())


async def get_transport() -> ConsoleTransport | NATSClient | None:
    global _client
    if _client is not None:
        return _client

    if not settings.nats_host:
        return ConsoleTransport()

    nats_url = (
        settings.nats_host
        if settings.nats_host.startswith(("nats://", "tls://"))
        else f"nats://{settings.nats_host}"
    )

    try:
        import nats

        nc = await nats.connect(nats_url)
        _client = nc
        return nc
    except Exception as exc:  # pragma: no cover
        logger.warning("NATS unavailable, using console transport: %s", exc)
        return ConsoleTransport()


async def close_transport() -> None:
    global _client
    if _client is not None:
        await _client.close()
        _client = None
