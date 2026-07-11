"""Rule-based career guidance engine with explainability."""

from collections import defaultdict
from datetime import UTC, datetime, timedelta
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from career_guidance_service.models import FeatureStoreMetric, Guidance

_LOOKBACK_DAYS = 180
_CONFIDENCE_BASE = 0.4
_CONFIDENCE_STEP = 0.11
_CONFIDENCE_CAP = 0.95
_CONTRIBUTION_ROUND = 2
_STRONG_NEGATIVE_THRESHOLD = -0.2
_NEGATIVE_THRESHOLD = -0.05
_POSITIVE_THRESHOLD = 0.05
_STRONG_POSITIVE_THRESHOLD = 0.2
_ATTENDANCE_THRESHOLD = 0.75
_SCORE_THRESHOLD = 60.0
_COMPLETION_THRESHOLD = 0.7
_CAREER_TRACK_STEM_THRESHOLD = 75
_CAREER_TRACK_GENERAL_THRESHOLD = 60
_CAREER_ATTENDANCE_STEM_THRESHOLD = 0.8
_COURSE_LOAD_MIN = 4
_COURSE_LOAD_RANGE = 3
_STUDY_STRATEGY_LOW_ATTENDANCE = 0.7
_STUDY_STRATEGY_LOW_COMPLETION = 0.6
_STUDY_STRATEGY_FOCUS_COMPLETION = 0.8


def _average(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0.0


async def fetch_student_metrics(
    session: AsyncSession,
    tenant_id: str,
    student_id: str,
) -> list[FeatureStoreMetric]:
    since = datetime.now(UTC) - timedelta(days=_LOOKBACK_DAYS)
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
    return round(min(_CONFIDENCE_BASE + sample_count * _CONFIDENCE_STEP, _CONFIDENCE_CAP), 2)


def _contribution(value: float, threshold: float, higher_is_better: bool) -> str:
    diff = (value - threshold) / threshold if threshold else 0.0
    sign = 1 if higher_is_better else -1
    signed_diff = diff * sign
    if signed_diff <= _STRONG_NEGATIVE_THRESHOLD:
        return "strong_negative"
    if signed_diff < _NEGATIVE_THRESHOLD:
        return "negative"
    if signed_diff >= _STRONG_POSITIVE_THRESHOLD:
        return "strong_positive"
    if signed_diff > _POSITIVE_THRESHOLD:
        return "positive"
    return "neutral"


def _career_track(avg_score: float, avg_attendance: float) -> str:
    stem_ready = (
        avg_score >= _CAREER_TRACK_STEM_THRESHOLD
        and avg_attendance >= _CAREER_ATTENDANCE_STEM_THRESHOLD
    )
    if stem_ready:
        return "STEM / Health Sciences"
    if avg_score >= _CAREER_TRACK_GENERAL_THRESHOLD:
        return "Business / Arts / Social Sciences"
    return "Foundation / Vocational / Support pathway"


def _study_strategy(avg_attendance: float, avg_completion: float) -> str:
    needs_attention = (
        avg_attendance < _STUDY_STRATEGY_LOW_ATTENDANCE
        or avg_completion < _STUDY_STRATEGY_LOW_COMPLETION
    )
    if needs_attention:
        return "Increase attendance and complete missing assignments"
    if avg_completion < _STUDY_STRATEGY_FOCUS_COMPLETION:
        return "Focus on consistent assignment completion"
    return "Maintain current study habits; consider advanced material"


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
        avg_attendance = _average(attendance_values) if attendance_values else _ATTENDANCE_THRESHOLD
        factors = [
            {
                "metric_key": "average_score",
                "value": round(avg_score, _CONTRIBUTION_ROUND),
                "contribution": _contribution(avg_score, _SCORE_THRESHOLD, True),
            }
        ]
        if attendance_values:
            factors.append(
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, _CONTRIBUTION_ROUND),
                    "contribution": _contribution(avg_attendance, _ATTENDANCE_THRESHOLD, True),
                }
            )
        track = _career_track(avg_score, avg_attendance)
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
        avg_completion = _average(completion_values) if completion_values else _COMPLETION_THRESHOLD
        # Recommend 4-7 courses based on readiness.
        readiness = min(1.0, (avg_score / 100.0) * 0.6 + avg_completion * 0.4)
        recommended_courses = int(_COURSE_LOAD_MIN + readiness * _COURSE_LOAD_RANGE)
        factors = []
        if score_values:
            factors.append(
                {
                    "metric_key": "average_score",
                    "value": round(avg_score, _CONTRIBUTION_ROUND),
                    "contribution": _contribution(avg_score, _SCORE_THRESHOLD, True),
                }
            )
        if completion_values:
            factors.append(
                {
                    "metric_key": "assignment_completion_rate",
                    "value": round(avg_completion, _CONTRIBUTION_ROUND),
                    "contribution": _contribution(avg_completion, _COMPLETION_THRESHOLD, True),
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
        avg_attendance = _average(attendance_values) if attendance_values else _ATTENDANCE_THRESHOLD
        avg_completion = _average(completion_values) if completion_values else _COMPLETION_THRESHOLD
        strategy = _study_strategy(avg_attendance, avg_completion)
        factors = []
        if attendance_values:
            factors.append(
                {
                    "metric_key": "attendance_rate",
                    "value": round(avg_attendance, _CONTRIBUTION_ROUND),
                    "contribution": _contribution(avg_attendance, _ATTENDANCE_THRESHOLD, True),
                }
            )
        if completion_values:
            factors.append(
                {
                    "metric_key": "assignment_completion_rate",
                    "value": round(avg_completion, _CONTRIBUTION_ROUND),
                    "contribution": _contribution(avg_completion, _COMPLETION_THRESHOLD, True),
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
) -> dict[str, Any]:
    """Build explainability payload for guidance."""
    metrics = await fetch_student_metrics(
        session,
        guidance.tenant_id,
        guidance.student_id,
    )
    buckets = _aggregate_metrics(metrics)
    factors: list[dict[str, Any]] = []

    thresholds = {
        "attendance_rate": (_ATTENDANCE_THRESHOLD, True),
        "average_score": (_SCORE_THRESHOLD, True),
        "assessment_score": (_SCORE_THRESHOLD, True),
        "assignment_completion_rate": (_COMPLETION_THRESHOLD, True),
    }

    for key, values in buckets.items():
        if not values:
            continue
        avg = _average(values)
        threshold, higher_is_better = thresholds.get(key, (avg, True))
        factors.append(
            {
                "metric_key": key,
                "value": round(avg, _CONTRIBUTION_ROUND),
                "contribution": _contribution(avg, threshold, higher_is_better),
            }
        )

    return {
        "guidance_id": guidance.id,
        "factors": factors,
        "model_notes": guidance.explanation,
    }
