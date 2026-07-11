"""AI Guidance Service API routes."""

from datetime import UTC, datetime

from fastapi import APIRouter, HTTPException, status
from sqlalchemy import select

from career_guidance_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_feature_enabled,
)
from career_guidance_service.domain.engine import build_explanation, generate_guidance
from career_guidance_service.events import publisher
from career_guidance_service.models import FeatureStoreMetric, Guidance
from career_guidance_service.schemas import (
    ExplainResponse,
    FeatureStoreMetricSchema,
    GenerateGuidanceRequest,
    GuidanceList,
    Health,
    IngestMetricRequest,
    ReviewGuidanceRequest,
)
from career_guidance_service.schemas import (
    Guidance as GuidanceSchema,
)

public_router = APIRouter(tags=["guidance"])
router = APIRouter(tags=["guidance"], dependencies=[require_feature_enabled("career_guidance")])


@public_router.get("/health", response_model=Health)
async def health_check() -> Health:
    return Health(status="ok")


@router.post("/feature-store/metrics", status_code=status.HTTP_201_CREATED)
async def ingest_metric(
    body: IngestMetricRequest,
    tenant_id: TenantId,
    db: DbSession,
) -> FeatureStoreMetricSchema:
    metric = FeatureStoreMetric(
        tenant_id=tenant_id,
        student_id=body.student_id,
        metric_key=body.metric_key,
        value=body.value,
        source=body.source,
        recorded_at=body.recorded_at,
    )
    db.add(metric)
    await db.flush()
    return FeatureStoreMetricSchema.model_validate(metric)


@router.get("/guidance", response_model=GuidanceList)
async def list_guidance(
    tenant_id: TenantId,
    student_id: str,
    db: DbSession,
    guidance_type: str | None = None,
) -> GuidanceList:
    stmt = select(Guidance).where(
        Guidance.tenant_id == tenant_id,
        Guidance.student_id == student_id,
    )
    if guidance_type:
        stmt = stmt.where(Guidance.guidance_type == guidance_type)
    stmt = stmt.order_by(Guidance.created_at.desc())
    result = await db.execute(stmt)
    items = result.scalars().all()
    return GuidanceList(
        data=[GuidanceSchema.model_validate(item) for item in items],
    )


@router.post("/guidance", status_code=status.HTTP_201_CREATED, response_model=GuidanceList)
async def create_guidance(
    body: GenerateGuidanceRequest,
    tenant_id: TenantId,
    actor: CurrentActor,
    db: DbSession,
) -> GuidanceList:
    guidance = await generate_guidance(
        db,
        tenant_id,
        body.student_id,
        body.guidance_types,
    )
    for item in guidance:
        db.add(item)
    await db.flush()

    if publisher.publish_guidance is not None:
        await publisher.publish_guidance(tenant_id, actor.user_id, guidance)

    return GuidanceList(
        data=[GuidanceSchema.model_validate(p) for p in guidance],
    )


@router.get("/guidance/{guidance_id}", response_model=GuidanceSchema)
async def get_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await ensure_tenant_match(tenant_id, guidance.tenant_id)
    return GuidanceSchema.model_validate(guidance)


@router.get("/guidance/{guidance_id}/explain", response_model=ExplainResponse)
async def explain_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
) -> ExplainResponse:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await ensure_tenant_match(tenant_id, guidance.tenant_id)
    payload = await build_explanation(db, guidance)
    return ExplainResponse.model_validate(payload)


@router.post("/guidance/{guidance_id}/approve", response_model=GuidanceSchema)
async def approve_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    _body: ReviewGuidanceRequest | None = None,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await ensure_tenant_match(tenant_id, guidance.tenant_id)
    guidance.status = "approved"
    guidance.updated_at = datetime.now(UTC)
    await db.flush()
    return GuidanceSchema.model_validate(guidance)


@router.post("/guidance/{guidance_id}/reject", response_model=GuidanceSchema)
async def reject_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    body: ReviewGuidanceRequest | None = None,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await ensure_tenant_match(tenant_id, guidance.tenant_id)
    guidance.status = "rejected"
    if body and body.reason:
        guidance.explanation = f"Rejected: {body.reason}"
    guidance.updated_at = datetime.now(UTC)
    await db.flush()
    return GuidanceSchema.model_validate(guidance)
