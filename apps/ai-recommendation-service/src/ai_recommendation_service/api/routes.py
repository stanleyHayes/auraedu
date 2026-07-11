"""HTTP API routes for the AI Recommendation Service."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, HTTPException, Query, status
from pydantic import BaseModel, ConfigDict, Field
from sqlalchemy import select

from ai_recommendation_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_permission,
)
from ai_recommendation_service.domain.engine import build_explanation, generate_recommendations
from ai_recommendation_service.events.publisher import RecommendationPublisher
from ai_recommendation_service.models import FeatureStoreMetric, Recommendation

router = APIRouter()


class IngestMetricRequest(BaseModel):
    student_id: UUID
    metric_key: str = Field(..., min_length=1, max_length=128)
    value: float
    source: str = Field(..., pattern="^(assessment|attendance|analytics)$")
    recorded_at: datetime


class FeatureStoreMetricResponse(BaseModel):
    id: UUID
    tenant_id: UUID
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
    tenant_id: UUID
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


@router.get("/health", status_code=status.HTTP_200_OK)
async def health_check() -> dict[str, str]:
    return {"status": "ok"}


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
    student_id: UUID,
    db: DbSession,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
) -> RecommendationListResponse:
    stmt = select(Recommendation).where(
        Recommendation.tenant_id == str(tenant_id),
        Recommendation.student_id == str(student_id),
    )
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
) -> RecommendationListResponse:
    recs = await generate_recommendations(
        db,
        str(tenant_id),
        str(payload.student_id),
        payload.recommendation_types,
    )
    db.add_all(recs)
    await db.flush()
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
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
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
    _payload: RejectRecommendationRequest | None = None,
) -> Recommendation:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
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
) -> ExplainResponse:
    rec = await db.get(Recommendation, str(recommendation_id))
    if rec is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Recommendation not found"},
        )
    await ensure_tenant_match(str(tenant_id), rec.tenant_id)
    explanation = await build_explanation(db, rec)
    return ExplainResponse(**explanation)
