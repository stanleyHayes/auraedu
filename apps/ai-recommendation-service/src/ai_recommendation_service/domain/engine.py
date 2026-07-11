"""Rule-based recommendation engine with explainability."""

from collections import defaultdict
from datetime import UTC, datetime, timedelta

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ai_recommendation_service.models import FeatureStoreMetric, Recommendation


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
    """Simple confidence curve: more samples -> higher confidence, capped at 0.95."""
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


async def generate_recommendations(
    session: AsyncSession,
    tenant_id: str,
    student_id: str,
    requested_types: list[str] | None = None,
) -> list[Recommendation]:
    """Generate pending recommendations for a student from feature store metrics."""
    metrics = await fetch_student_metrics(session, tenant_id, student_id)
    buckets = _aggregate_metrics(metrics)
    recommendations: list[Recommendation] = []

    def include(rec_type: str) -> bool:
        return requested_types is None or rec_type in requested_types

    # Attendance intervention
    attendance_values = buckets.get("attendance_rate", [])
    if attendance_values and include("attendance_intervention"):
        avg_attendance = sum(attendance_values) / len(attendance_values)
        if avg_attendance < 0.75:
            factors = [
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, 2),
                    "contribution": _contribution(avg_attendance, 0.75, True),
                }
            ]
            recommendations.append(
                Recommendation(
                    tenant_id=tenant_id,
                    student_id=student_id,
                    recommendation_type="attendance_intervention",
                    title="Improve attendance",
                    description=(
                        "Recent attendance is below 75%. Consistent attendance "
                        "is strongly correlated with improved academic outcomes."
                    ),
                    confidence=_confidence(len(attendance_values)),
                    explanation=f"Factors: {factors}",
                    status="pending",
                )
            )

    # Academic support
    score_values = buckets.get("average_score", []) + buckets.get("assessment_score", [])
    if score_values and include("academic_support"):
        avg_score = sum(score_values) / len(score_values)
        if avg_score < 60.0:
            factors = [
                {
                    "metric_key": "average_score",
                    "value": round(avg_score, 2),
                    "contribution": _contribution(avg_score, 60.0, True),
                }
            ]
            recommendations.append(
                Recommendation(
                    tenant_id=tenant_id,
                    student_id=student_id,
                    recommendation_type="academic_support",
                    title="Extra academic support",
                    description=(
                        "Average score is below 60%. Consider remedial classes, "
                        "peer tutoring, or one-on-one teacher support."
                    ),
                    confidence=_confidence(len(score_values)),
                    explanation=f"Factors: {factors}",
                    status="pending",
                )
            )

    # Assignment completion
    completion_values = buckets.get("assignment_completion_rate", [])
    if completion_values and include("assignment_completion"):
        avg_completion = sum(completion_values) / len(completion_values)
        if avg_completion < 0.7:
            factors = [
                {
                    "metric_key": "assignment_completion_rate",
                    "value": round(avg_completion, 2),
                    "contribution": _contribution(avg_completion, 0.7, True),
                }
            ]
            recommendations.append(
                Recommendation(
                    tenant_id=tenant_id,
                    student_id=student_id,
                    recommendation_type="assignment_completion",
                    title="Complete assignments on time",
                    description=(
                        "Assignment completion rate is below 70%. Building a consistent "
                        "homework routine may improve understanding and grades."
                    ),
                    confidence=_confidence(len(completion_values)),
                    explanation=f"Factors: {factors}",
                    status="pending",
                )
            )

    # Low-engagement fallback when no specific trigger fires
    if not recommendations and include("general_check_in"):
        recommendations.append(
            Recommendation(
                tenant_id=tenant_id,
                student_id=student_id,
                recommendation_type="general_check_in",
                title="Schedule a check-in",
                description=(
                    "No strong risk signals detected, but a periodic teacher check-in "
                    "helps maintain progress and catch issues early."
                ),
                confidence=_confidence(len(metrics)),
                explanation="Factors: no strong negative signals in recent data.",
                status="pending",
            )
        )

    return recommendations


async def build_explanation(
    session: AsyncSession,
    recommendation: Recommendation,
) -> dict:
    """Build explainability payload for a recommendation."""
    metrics = await fetch_student_metrics(
        session,
        recommendation.tenant_id,
        recommendation.student_id,
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
        avg = sum(values) / len(values)
        threshold, higher_is_better = thresholds.get(key, (avg, True))
        factors.append(
            {
                "metric_key": key,
                "value": round(avg, 2),
                "contribution": _contribution(avg, threshold, higher_is_better),
            }
        )

    return {
        "recommendation_id": recommendation.id,
        "factors": factors,
        "model_notes": recommendation.explanation,
    }
