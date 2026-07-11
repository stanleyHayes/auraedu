"""Rule-based career guidance engine with explainability."""

from collections import defaultdict
from datetime import UTC, datetime, timedelta

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from career_guidance_service.models import FeatureStoreMetric, Guidance


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


async def generate_guidance(
    session: AsyncSession,
    tenant_id: str,
    student_id: str,
    requested_types: list[str] | None = None,
) -> list[Guidance]:
    """Generate career guidance for a student from feature store metrics."""
    metrics = await fetch_student_metrics(session, tenant_id, student_id)
    buckets = _aggregate_metrics(metrics)
    guidance: list[Guidance] = []

    def include(guidance_type: str) -> bool:
        return requested_types is None or guidance_type in requested_types

    score_values = buckets.get("average_score", []) + buckets.get("assessment_score", [])
    attendance_values = buckets.get("attendance_rate", [])
    completion_values = buckets.get("assignment_completion_rate", [])

    # Career track recommendation
    if include("career_track") and score_values:
        avg_score = _average(score_values)
        avg_attendance = _average(attendance_values) if attendance_values else 0.75
        factors = [
            {
                "metric_key": "average_score",
                "value": round(avg_score, 2),
                "contribution": _contribution(avg_score, 60.0, True),
            }
        ]
        if attendance_values:
            factors.append(
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, 2),
                    "contribution": _contribution(avg_attendance, 0.75, True),
                }
            )
        if avg_score >= 75 and avg_attendance >= 0.8:
            track = "STEM / Health Sciences"
        elif avg_score >= 60:
            track = "Business / Arts / Social Sciences"
        else:
            track = "Foundation / Vocational / Support pathway"
        guidance.append(
            Guidance(
                tenant_id=tenant_id,
                student_id=student_id,
                guidance_type="career_track",
                title="Recommended career track",
                value=round(min(avg_score / 100.0, 1.0), 2),
                confidence=_confidence(len(score_values) + len(attendance_values)),
                explanation=f"Recommended track: {track}. Factors: {factors}",
            )
        )

    # Course load recommendation
    if include("course_load") and (score_values or completion_values):
        avg_score = _average(score_values) if score_values else 50.0
        avg_completion = _average(completion_values) if completion_values else 0.7
        # Recommend 4-7 courses based on readiness.
        readiness = min(1.0, (avg_score / 100.0) * 0.6 + avg_completion * 0.4)
        recommended_courses = int(4 + readiness * 3)
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
        guidance.append(
            Guidance(
                tenant_id=tenant_id,
                student_id=student_id,
                guidance_type="course_load",
                title="Recommended course load",
                value=float(recommended_courses),
                confidence=_confidence(len(score_values) + len(completion_values)),
                explanation=f"Recommended {recommended_courses} courses. Factors: {factors}",
            )
        )

    # Study strategy recommendation
    if include("study_strategy") and (attendance_values or completion_values):
        avg_attendance = _average(attendance_values) if attendance_values else 0.75
        avg_completion = _average(completion_values) if completion_values else 0.7
        if avg_attendance < 0.7 or avg_completion < 0.6:
            strategy = "Increase attendance and complete missing assignments"
        elif avg_completion < 0.8:
            strategy = "Focus on consistent assignment completion"
        else:
            strategy = "Maintain current study habits; consider advanced material"
        factors = []
        if attendance_values:
            factors.append(
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, 2),
                    "contribution": _contribution(avg_attendance, 0.75, True),
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
        guidance.append(
            Guidance(
                tenant_id=tenant_id,
                student_id=student_id,
                guidance_type="study_strategy",
                title="Study strategy",
                value=round(min(avg_attendance * 0.5 + avg_completion * 0.5, 1.0), 2),
                confidence=_confidence(len(attendance_values) + len(completion_values)),
                explanation=f"Strategy: {strategy}. Factors: {factors}",
            )
        )

    if not guidance:
        guidance.append(
            Guidance(
                tenant_id=tenant_id,
                student_id=student_id,
                guidance_type="insufficient_data",
                title="Insufficient data",
                value=0.0,
                confidence=0.0,
                explanation="Not enough recent metrics to generate career guidance.",
            )
        )

    return guidance


async def build_explanation(
    session: AsyncSession,
    guidance: Guidance,
) -> dict:
    """Build explainability payload for guidance."""
    metrics = await fetch_student_metrics(
        session,
        guidance.tenant_id,
        guidance.student_id,
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
        "guidance_id": guidance.id,
        "factors": factors,
        "model_notes": guidance.explanation,
    }
