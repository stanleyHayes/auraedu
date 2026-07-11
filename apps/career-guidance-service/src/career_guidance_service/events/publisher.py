"""CloudEvent publisher for generated career guidance."""

import json
import uuid
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from career_guidance_service.config import settings
from career_guidance_service.events.transport import get_transport

if TYPE_CHECKING:
    from career_guidance_service.models import Guidance


async def publish_guidance(
    tenant_id: str,
    actor_user_id: str | None,
    guidance_items: list["Guidance"],
) -> None:
    transport = await get_transport()
    if transport is None:
        return

    now = datetime.now(UTC).isoformat()
    for item in guidance_items:
        event = {
            "specversion": "1.0",
            "type": "ai.guidance_generated.v1",
            "source": settings.service_name,
            "id": str(uuid.uuid4()),
            "time": now,
            "tenant_id": tenant_id,
            "datacontenttype": "application/json",
            "subject": f"students/{item.student_id}",
            "data": {
                "student_id": item.student_id,
                "guidance_type": item.guidance_type,
                "confidence": item.confidence,
                "explanation": item.explanation,
            },
        }
        await transport.publish("ai.guidance_generated.v1", json.dumps(event).encode())
