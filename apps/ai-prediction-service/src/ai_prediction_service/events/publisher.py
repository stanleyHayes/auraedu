"""CloudEvent publisher for generated predictions."""

import json
import uuid
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from ai_prediction_service.config import settings
from ai_prediction_service.events.transport import get_transport

if TYPE_CHECKING:
    from ai_prediction_service.models import Prediction


async def publish_predictions(
    tenant_id: str,
    _actor_user_id: str | None,
    predictions: list[Prediction],
) -> None:
    transport = await get_transport()

    now = datetime.now(UTC).isoformat()
    for prediction in predictions:
        event = {
            "specversion": "1.0",
            "type": "ai.prediction_generated.v1",
            "source": settings.service_name,
            "id": str(uuid.uuid4()),
            "time": now,
            "tenant_id": tenant_id,
            "datacontenttype": "application/json",
            "subject": f"students/{prediction.student_id}",
            "data": {
                "student_id": prediction.student_id,
                "prediction_type": prediction.prediction_type,
                "confidence": prediction.confidence,
                "explanation": prediction.explanation,
            },
        }
        await transport.publish("ai.prediction_generated.v1", json.dumps(event).encode())
