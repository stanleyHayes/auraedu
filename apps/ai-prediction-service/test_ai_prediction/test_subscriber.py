"""Tests for the AI Prediction feature-store event subscriber."""

from ai_prediction_service.events.subscriber import process_event
from ai_prediction_service.models import FeatureStoreMetric
from sqlalchemy import select


async def test_process_assessment_score_event(db_session):
    event = {
        "type": "assessment.score_recorded.v1",
        "tenant_id": "tenant-a",
        "data": {
            "student_id": "66666666-6666-6666-6666-666666666666",
            "score": 75,
            "max_score": 100,
        },
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.tenant_id == "tenant-a",
            FeatureStoreMetric.student_id == "66666666-6666-6666-6666-666666666666",
        )
    )
    metric = result.scalar_one()
    assert metric.metric_key == "assessment_score"
    assert metric.value == 0.75
    assert metric.source == "assessment"


async def test_process_attendance_event(db_session):
    event = {
        "type": "attendance.marked",
        "tenant_id": "tenant-a",
        "data": {
            "student_id": "77777777-7777-7777-7777-777777777777",
            "status": "absent",
        },
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.tenant_id == "tenant-a",
            FeatureStoreMetric.student_id == "77777777-7777-7777-7777-777777777777",
        )
    )
    metric = result.scalar_one()
    assert metric.metric_key == "attendance_rate"
    assert metric.value == 0.0


async def test_unsupported_event_is_ignored(db_session):
    event = {
        "type": "payment.received",
        "tenant_id": "tenant-a",
        "data": {"student_id": "88888888-8888-8888-8888-888888888888"},
    }
    await process_event(event)
    await db_session.commit()

    result = await db_session.execute(
        select(FeatureStoreMetric).where(
            FeatureStoreMetric.student_id == "88888888-8888-8888-8888-888888888888"
        )
    )
    assert result.scalar_one_or_none() is None
