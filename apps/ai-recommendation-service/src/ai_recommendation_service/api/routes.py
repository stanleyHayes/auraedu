"""HTTP API routes for the AI Recommendation Service."""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, HTTPException, Query, status
from pydantic import BaseModel, ConfigDict, Field
from sqlalchemy import select, text

from ai_recommendation_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_feature_enabled,
    require_permission,
)
from ai_recommendation_service.db import engine
from ai_recommendation_service.domain.engine import build_explanation, generate_recommendations
from ai_recommendation_service.events.publisher import (
    RecommendationPublisher,
    recommendation_event_data,
)
from ai_recommendation_service.learner_scope import (
    LearnerScopeUnavailableError,
    resolve_learner_ids,
)
from ai_recommendation_service.models import (
    FeatureStoreMetric,
    Recommendation,
    RecommendationOutbox,
)

public_router = APIRouter()
router = APIRouter(dependencies=[require_feature_enabled("ai_recommendations")])


class IngestMetricRequest(BaseModel):
    student_id: UUID
    metric_key: str = Field(..., min_length=1, max_length=128)
    value: float
    source: str = Field(..., pattern="^(assessment|attendance|analytics)$")
    recorded_at: datetime


class FeatureStoreMetricResponse(BaseModel):
    id: UUID
    tenant_id: str
    student_id: UUID
    metric_key: str
    value: float
    source: str
    recorded_at: datetime
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)


class GenerateRecommendationsRequest(BaseModel):
    student_id: UUID
    recommendation_types: list[str] | None = None


class ApproveRecommendationRequest(BaseModel):
    note: str | None = None


class RejectRecommendationRequest(BaseModel):
    reason: str | None = None


class OverrideRecommendationRequest(BaseModel):
    title: str = Field(..., min_length=1, max_length=255)
    description: str | None = None
    note: str | None = None


class RecommendationResponse(BaseModel):
    id: UUID
    tenant_id: str
    student_id: UUID
    recommendation_type: str
    title: str
    description: str | None
    status: str
    confidence: float
    explanation: str | None
    approved_by: UUID | None
    approved_at: datetime | None
    created_at: datetime
    updated_at: datetime

    model_config = ConfigDict(from_attributes=True)


class RecommendationListResponse(BaseModel):
    data: list[RecommendationResponse]
    next_cursor: str | None = None


class ExplainFactorResponse(BaseModel):
    metric_key: str
    value: float
    contribution: str


class ExplainResponse(BaseModel):
    recommendation_id: UUID
    factors: list[ExplainFactorResponse]
    model_notes: str | None


def _publisher() -> RecommendationPublisher:
    from ai_recommendation_service.main import app_publisher  # noqa: PLC0415

    return app_publisher


async def _authorized_student_id(
    tenant_id: str,
    actor: CurrentActor,
    requested_student_id: UUID | None,
) -> str:
    """Resolve learner ownership or assignment for learner-facing actors."""
    if actor.tenant_id and actor.tenant_id != tenant_id:
        raise HTTPException(
            status_code=403,
            detail={"code": "tenant_mismatch", "message": "Actor tenant mismatch"},
        )
    if actor.role not in {"student", "parent", "teacher"}:
        if requested_student_id is None:
            raise HTTPException(
                status_code=422,
                detail={"code": "validation_error", "message": "student_id is required"},
            )
        return str(requested_student_id)
    if not actor.user_id:
        raise HTTPException(
            status_code=401,
            detail={"code": "unauthorized", "message": "Authenticated user is required"},
        )
    try:
        allowed = await resolve_learner_ids(tenant_id, actor.user_id, actor.role)
    except LearnerScopeUnavailableError as exc:
        raise HTTPException(
            status_code=503,
            detail={"code": "unavailable", "message": str(exc)},
        ) from exc
    if requested_student_id is None:
        if actor.role == "student" and len(allowed) == 1:
            return next(iter(allowed))
        raise HTTPException(
            status_code=422,
            detail={"code": "validation_error", "message": "student_id is required"},
        )
    if str(requested_student_id) not in allowed:
        # Hide learner identifiers that are outside the authenticated scope.
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    return str(requested_student_id)


@public_router.get("/health", status_code=status.HTTP_200_OK)
async def health_check() -> dict[str, str]:
    return {"status": "ok"}


async def database_ready() -> bool:
    try:
        async with asyncio.timeout(2):
            async with engine.connect() as connection:
                await connection.execute(text("SELECT 1"))
    except Exception:
        return False
    else:
        return True


@public_router.get("/ready", status_code=status.HTTP_200_OK)
async def readiness_check() -> dict[str, str]:
    if not await database_ready():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail={"code": "not_ready", "message": "Database dependency is unavailable"},
        )
    return {"status": "ready"}


@router.post(
    "/feature-store/metrics",
    response_model=FeatureStoreMetricResponse,
    status_code=status.HTTP_201_CREATED,
    dependencies=[require_permission("ai.view_recommendations")],
)
async def ingest_metric(
    tenant_id: TenantId,
    payload: IngestMetricRequest,
    db: DbSession,
) -> FeatureStoreMetric:
    metric = FeatureStoreMetric(
        tenant_id=str(tenant_id),
        student_id=str(payload.student_id),
        metric_key=payload.metric_key,
        value=payload.value,
        source=payload.source,
        recorded_at=payload.recorded_at,
    )
    db.add(metric)
    await db.flush()
    await db.refresh(metric)
    return metric


@router.get(
    "/recommendations",
    response_model=RecommendationListResponse,
    dependencies=[require_permission("ai.view_recommendations")],
)
async def list_recommendations(
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    student_id: UUID | None = None,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
) -> RecommendationListResponse:
    resolved_student_id = await _authorized_student_id(str(tenant_id), actor, student_id)
    if actor.role in {"student", "parent"} and status_filter not in {
        None,
        "approved",
        "overridden",
    }:
        raise HTTPException(
            status_code=403,
            detail={"code": "forbidden", "message": "Only approved recommendations are visible"},
        )
    stmt = select(Recommendation).where(
        Recommendation.tenant_id == str(tenant_id),
        Recommendation.student_id == resolved_student_id,
    )
    if actor.role in {"student", "parent"} and status_filter is None:
        stmt = stmt.where(Recommendation.status.in_(["approved", "overridden"]))
    if status_filter:
        stmt = stmt.where(Recommendation.status == status_filter)
    result = await db.execute(stmt.order_by(Recommendation.created_at.desc()))
    items = result.scalars().all()
    return RecommendationListResponse(
        data=[RecommendationResponse.model_validate(item) for item in items],
    )


@router.post(
    "/recommendations",
    response_model=RecommendationListResponse,
    status_code=status.HTTP_201_CREATED,
    dependencies=[require_permission("ai.view_recommendations")],
)
async def create_recommendations(
    tenant_id: TenantId,
    payload: GenerateRecommendationsRequest,
    db: DbSession,
    actor: CurrentActor,
) -> RecommendationListResponse:
    student_id = await _authorized_student_id(str(tenant_id), actor, payload.student_id)
    recs = await generate_recommendations(
        db,
        str(tenant_id),
        student_id,
        payload.recommendation_types,
    )
    db.add_all(recs)
    await db.flush()
    if db.get_bind().dialect.name == "postgresql":
        for rec in recs:
            db.add(
                RecommendationOutbox(
                    tenant_id=str(tenant_id),
                    event_type="ai.recommendation_generated.v1",
                    payload=recommendation_event_data(rec),
                )
            )
        await db.flush()
    else:
        publisher = _publisher()
        for rec in recs:
            await publisher.publish_recommendation_generated(str(tenant_id), rec)
    return RecommendationListResponse(
        data=[RecommendationResponse.model_validate(rec) for rec in recs],
    )


@router.get(
    "/recommendations/{recommendation_id}",
    response_model=RecommendationResponse,
    dependencies=[require_permission("ai.view_recommendations")],
)
async def get_recommendation(
    tenant_id: TenantId,
    recommendation_id: UUID,
    db: DbSession,
    actor: CurrentActor,
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    await _authorized_student_id(str(tenant_id), actor, UUID(rec.student_id))
    if actor.role in {"student", "parent"} and rec.status not in {"approved", "overridden"}:
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    return rec


@router.post(
    "/recommendations/{recommendation_id}/approve",
    response_model=RecommendationResponse,
    dependencies=[require_permission("ai.approve_recommendations")],
)
async def approve_recommendation(
    tenant_id: TenantId,
    recommendation_id: UUID,
    db: DbSession,
    actor: CurrentActor,
    _payload: ApproveRecommendationRequest | None = None,
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    await _authorized_student_id(str(tenant_id), actor, UUID(rec.student_id))
    rec.status = "approved"
    rec.approved_by = actor.user_id
    rec.approved_at = datetime.now(UTC)
    await db.flush()
    return rec


@router.post(
    "/recommendations/{recommendation_id}/reject",
    response_model=RecommendationResponse,
    dependencies=[require_permission("ai.approve_recommendations")],
)
async def reject_recommendation(
    tenant_id: TenantId,
    recommendation_id: UUID,
    db: DbSession,
    actor: CurrentActor,
    _payload: RejectRecommendationRequest | None = None,
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    await _authorized_student_id(str(tenant_id), actor, UUID(rec.student_id))
    rec.status = "rejected"
    await db.flush()
    return rec


@router.post(
    "/recommendations/{recommendation_id}/override",
    response_model=RecommendationResponse,
    dependencies=[require_permission("ai.approve_recommendations")],
)
async def override_recommendation(
    tenant_id: TenantId,
    recommendation_id: UUID,
    payload: OverrideRecommendationRequest,
    db: DbSession,
    actor: CurrentActor,
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    await _authorized_student_id(str(tenant_id), actor, UUID(rec.student_id))
    rec.title = payload.title
    rec.description = payload.description or rec.description
    rec.status = "overridden"
    rec.approved_by = actor.user_id
    rec.approved_at = datetime.now(UTC)
    note = payload.note
    if note:
        rec.explanation = f"{rec.explanation or ''}\nTeacher override note: {note}".strip()
    await db.flush()
    return rec


@router.get(
    "/recommendations/{recommendation_id}/explain",
    response_model=ExplainResponse,
    dependencies=[require_permission("ai.view_recommendations")],
)
async def explain_recommendation(
    tenant_id: TenantId,
    recommendation_id: UUID,
    db: DbSession,
    actor: CurrentActor,
) -> ExplainResponse:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    await _authorized_student_id(str(tenant_id), actor, UUID(rec.student_id))
    if actor.role in {"student", "parent"} and rec.status not in {"approved", "overridden"}:
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    explanation = await build_explanation(db, rec)
    return ExplainResponse(**explanation)
