"""Rule-based prediction engine with explainability."""

from collections import defaultdict
from datetime import UTC, datetime, timedelta

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ai_prediction_service.models import FeatureStoreMetric, Prediction


async def fetch_student_metrics(
    session: AsyncSession,
    tenant_id: str,
    student_id: str,
) -> list[FeatureStoreMetric]:
    since = datetime.now(UTC) - timedelta(days=180)
    stmt = (
        select(FeatureStoreMetric)
        .where(
            FeatureStoreMetric.tenant_id == tenant_id,
            FeatureStoreMetric.student_id == student_id,
            FeatureStoreMetric.recorded_at >= since,
        )
        .order_by(FeatureStoreMetric.recorded_at.desc())
    )
    result = await session.execute(stmt)
    return list(result.scalars().all())


def _aggregate_metrics(metrics: list[FeatureStoreMetric]) -> dict[str, list[float]]:
    buckets: dict[str, list[float]] = defaultdict(list)
    for metric in metrics:
        buckets[metric.metric_key].append(metric.value)
    return buckets


def _confidence(sample_count: int) -> float:
    return round(min(0.4 + sample_count * 0.11, 0.95), 2)


def _contribution(value: float, threshold: float, higher_is_better: bool) -> str:
    diff = (value - threshold) / threshold if threshold else 0.0
    if higher_is_better:
        if diff <= -0.2:
            return "strong_negative"
        if diff < -0.05:
            return "negative"
        if diff >= 0.2:
            return "strong_positive"
        if diff > 0.05:
            return "positive"
    else:
        if diff >= 0.2:
            return "strong_negative"
        if diff > 0.05:
            return "negative"
        if diff <= -0.2:
            return "strong_positive"
        if diff < -0.05:
            return "positive"
    return "neutral"


def _average(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0.0


async def generate_predictions(
    session: AsyncSession,
    tenant_id: str,
    student_id: str,
    requested_types: list[str] | None = None,
) -> list[Prediction]:
    """Generate predictions for a student from feature store metrics."""
    metrics = await fetch_student_metrics(session, tenant_id, student_id)
    buckets = _aggregate_metrics(metrics)
    predictions: list[Prediction] = []

    def include(pred_type: str) -> bool:
        return requested_types is None or pred_type in requested_types

    score_values = buckets.get("average_score", []) + buckets.get("assessment_score", [])
    attendance_values = buckets.get("attendance_rate", [])
    completion_values = buckets.get("assignment_completion_rate", [])

    # At-risk prediction
    if include("at_risk") and (score_values or attendance_values):
        avg_score = _average(score_values)
        avg_attendance = _average(attendance_values)
        risk_score = 0.0
        factors = []
        if score_values:
            score_factor = max(0.0, (60.0 - avg_score) / 60.0)
            risk_score += score_factor * 0.6
            factors.append(
                {
                    "metric_key": "average_score",
                    "value": round(avg_score, 2),
                    "contribution": _contribution(avg_score, 60.0, True),
                }
            )
        if attendance_values:
            attendance_factor = max(0.0, (0.75 - avg_attendance) / 0.75)
            risk_score += attendance_factor * 0.4
            factors.append(
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, 2),
                    "contribution": _contribution(avg_attendance, 0.75, True),
                }
            )
        predictions.append(
            Prediction(
                tenant_id=tenant_id,
                student_id=student_id,
                prediction_type="at_risk",
                title="At-risk score",
                value=round(min(risk_score, 1.0), 2),
                confidence=_confidence(len(score_values) + len(attendance_values)),
                explanation=f"Factors: {factors}",
            )
        )

    # Performance trend prediction
    if include("performance_trend") and len(score_values) >= 2:
        midpoint = len(score_values) // 2
        recent = _average(score_values[:midpoint])
        older = _average(score_values[midpoint:])
        delta = recent - older
        trend_value = round(delta, 2)
        factors = [
            {
                "metric_key": "average_score",
                "value": round(recent, 2),
                "contribution": _contribution(recent, older, True),
            }
        ]
        predictions.append(
            Prediction(
                tenant_id=tenant_id,
                student_id=student_id,
                prediction_type="performance_trend",
                title="Performance trend",
                value=trend_value,
                confidence=_confidence(len(score_values)),
                explanation=f"Recent average {recent:.1f} vs older {older:.1f}. Factors: {factors}",
            )
        )

    # Exam readiness prediction
    if include("exam_readiness") and (score_values or completion_values):
        avg_score = _average(score_values)
        avg_completion = _average(completion_values) if completion_values else avg_score / 100.0
        readiness = min(1.0, (avg_score / 100.0) * 0.7 + avg_completion * 0.3)
        factors = []
        if score_values:
            factors.append(
                {
                    "metric_key": "average_score",
                    "value": round(avg_score, 2),
                    "contribution": _contribution(avg_score, 60.0, True),
                }
            )
        if completion_values:
            factors.append(
                {
                    "metric_key": "assignment_completion_rate",
                    "value": round(avg_completion, 2),
                    "contribution": _contribution(avg_completion, 0.7, True),
                }
            )
        predictions.append(
            Prediction(
                tenant_id=tenant_id,
                student_id=student_id,
                prediction_type="exam_readiness",
                title="Exam readiness",
                value=round(readiness, 2),
                confidence=_confidence(len(score_values) + len(completion_values)),
                explanation=f"Factors: {factors}",
            )
        )

    if not predictions:
        predictions.append(
            Prediction(
                tenant_id=tenant_id,
                student_id=student_id,
                prediction_type="insufficient_data",
                title="Insufficient data",
                value=0.0,
                confidence=0.0,
                explanation="Not enough recent metrics to generate a reliable prediction.",
            )
        )

    return predictions


async def build_explanation(
    session: AsyncSession,
    prediction: Prediction,
) -> dict:
    """Build explainability payload for a prediction."""
    metrics = await fetch_student_metrics(
        session,
        prediction.tenant_id,
        prediction.student_id,
    )
    buckets = _aggregate_metrics(metrics)
    factors: list[dict] = []

    thresholds = {
        "attendance_rate": (0.75, True),
        "average_score": (60.0, True),
        "assessment_score": (60.0, True),
        "assignment_completion_rate": (0.7, True),
    }

    for key, values in buckets.items():
        if not values:
            continue
        avg = _average(values)
        threshold, higher_is_better = thresholds.get(key, (avg, True))
        factors.append(
            {
                "metric_key": key,
                "value": round(avg, 2),
                "contribution": _contribution(avg, threshold, higher_is_better),
            }
        )

    return {
        "prediction_id": prediction.id,
        "factors": factors,
        "model_notes": prediction.explanation,
    }
