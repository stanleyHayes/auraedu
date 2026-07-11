"""Deterministic evaluation tests for the AI Prediction engine."""

from datetime import UTC, datetime, timedelta

import pytest
from ai_prediction_service.domain.engine import generate_predictions
from ai_prediction_service.models import FeatureStoreMetric, Prediction
from sqlalchemy import select


@pytest.fixture
def high_performing_student(make_metric):
    async def _builder(session, tenant_id: str, student_id: str):
        session.add_all(
            [
                make_metric(tenant_id, student_id, "average_score", 85.0, days_ago=1),
                make_metric(tenant_id, student_id, "average_score", 88.0, days_ago=2),
                make_metric(tenant_id, student_id, "attendance_rate", 0.95, days_ago=1),
                make_metric(tenant_id, student_id, "assignment_completion_rate", 0.92, days_ago=1),
            ]
        )
        await session.commit()

    return _builder


@pytest.fixture
def at_risk_student(make_metric):
    async def _builder(session, tenant_id: str, student_id: str):
        session.add_all(
            [
                make_metric(tenant_id, student_id, "average_score", 45.0, days_ago=1),
                make_metric(tenant_id, student_id, "average_score", 50.0, days_ago=2),
                make_metric(tenant_id, student_id, "attendance_rate", 0.55, days_ago=1),
            ]
        )
        await session.commit()

    return _builder


@pytest.fixture
def make_metric():
    def _make(tenant_id: str, student_id: str, key: str, value: float, days_ago: int = 0):
        return FeatureStoreMetric(
            tenant_id=tenant_id,
            student_id=student_id,
            metric_key=key,
            value=value,
            source="assessment",
            recorded_at=datetime.now(UTC) - timedelta(days=days_ago),
        )

    return _make


async def test_at_risk_prediction_is_generated(db_session, at_risk_student):
    tenant_id = "tenant-a"
    student_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
    await at_risk_student(db_session, tenant_id, student_id)

    predictions = await generate_predictions(db_session, tenant_id, student_id)
    types = {p.prediction_type for p in predictions}
    assert "at_risk" in types

    at_risk = next(p for p in predictions if p.prediction_type == "at_risk")
    assert 0.0 <= at_risk.value <= 1.0
    assert 0.0 <= at_risk.confidence <= 1.0
    assert at_risk.value > 0.0


async def test_high_performer_has_low_at_risk_score(db_session, high_performing_student):
    tenant_id = "tenant-a"
    student_id = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
    await high_performing_student(db_session, tenant_id, student_id)

    predictions = await generate_predictions(db_session, tenant_id, student_id)
    at_risk = next((p for p in predictions if p.prediction_type == "at_risk"), None)
    assert at_risk is not None
    assert at_risk.value < 0.5


async def test_exam_readiness_for_high_performer(db_session, high_performing_student):
    tenant_id = "tenant-a"
    student_id = "cccccccc-cccc-cccc-cccc-cccccccccccc"
    await high_performing_student(db_session, tenant_id, student_id)

    predictions = await generate_predictions(db_session, tenant_id, student_id)
    readiness = next((p for p in predictions if p.prediction_type == "exam_readiness"), None)
    assert readiness is not None
    assert readiness.value >= 0.7
    assert readiness.confidence > 0.0


async def test_performance_trend_requires_multiple_scores(db_session, make_metric):
    tenant_id = "tenant-a"
    student_id = "dddddddd-dddd-dddd-dddd-dddddddddddd"
    db_session.add_all(
        [
            make_metric(tenant_id, student_id, "average_score", 60.0, days_ago=1),
            make_metric(tenant_id, student_id, "average_score", 70.0, days_ago=2),
            make_metric(tenant_id, student_id, "average_score", 80.0, days_ago=3),
        ]
    )
    await db_session.commit()

    predictions = await generate_predictions(db_session, tenant_id, student_id)
    types = {p.prediction_type for p in predictions}
    assert "performance_trend" in types


async def test_insufficient_data_when_no_metrics(db_session):
    tenant_id = "tenant-a"
    student_id = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
    predictions = await generate_predictions(db_session, tenant_id, student_id)
    assert len(predictions) == 1
    assert predictions[0].prediction_type == "insufficient_data"
    assert predictions[0].confidence == 0.0


async def test_requested_types_filter(db_session, high_performing_student):
    tenant_id = "tenant-a"
    student_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
    await high_performing_student(db_session, tenant_id, student_id)

    predictions = await generate_predictions(
        db_session, tenant_id, student_id, requested_types=["exam_readiness"]
    )
    assert {p.prediction_type for p in predictions} == {"exam_readiness"}


async def test_predictions_are_persisted(db_session, at_risk_student):
    tenant_id = "tenant-a"
    student_id = "11111111-2222-3333-4444-555555555555"
    await at_risk_student(db_session, tenant_id, student_id)

    predictions = await generate_predictions(db_session, tenant_id, student_id)
    for prediction in predictions:
        db_session.add(prediction)
    await db_session.commit()

    result = await db_session.execute(
        select(Prediction).where(
            Prediction.tenant_id == tenant_id,
            Prediction.student_id == student_id,
        )
    )
    stored = result.scalars().all()
    assert len(stored) == len(predictions)
