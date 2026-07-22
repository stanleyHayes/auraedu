"""Shared CloudEvent encoding limits for publisher and subscriber boundaries."""

from __future__ import annotations

import json
from collections.abc import Mapping
from typing import Any

MAX_EVENT_BYTES = 1 << 20


class EventTooLargeError(ValueError):
    """Raised before an oversized event envelope reaches the broker."""

    def __init__(self, actual_bytes: int) -> None:
        self.actual_bytes = actual_bytes
        super().__init__(f"event envelope is {actual_bytes} bytes; maximum is {MAX_EVENT_BYTES}")


def encode_event(event: Mapping[str, Any]) -> bytes:
    """Serialize a CloudEvent and enforce the platform envelope ceiling."""
    payload = json.dumps(event, ensure_ascii=False, separators=(",", ":")).encode()
    if len(payload) > MAX_EVENT_BYTES:
        raise EventTooLargeError(len(payload))
    return payload
