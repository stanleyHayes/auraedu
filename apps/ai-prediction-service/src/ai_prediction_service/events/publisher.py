"""CloudEvent publisher for generated predictions."""

from __future__ import annotations

import uuid
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Any

from ai_prediction_service.config import settings
from ai_prediction_service.events.envelope import encode_event
from ai_prediction_service.events.transport import get_transport

if TYPE_CHECKING:
    from ai_prediction_service.models import Prediction


def prediction_event_data(prediction: Prediction) -> dict[str, Any]:
    """Build the contract-owned data object for every publication path."""
    return {
        "student_id": prediction.student_id,
        "prediction_type": prediction.prediction_type,
        "value": prediction.value,
        "confidence": prediction.confidence,
        "explanation": prediction.explanation,
    }


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
            "data": prediction_event_data(prediction),
        }
        await transport.publish("ai.prediction_generated.v1", encode_event(event))
