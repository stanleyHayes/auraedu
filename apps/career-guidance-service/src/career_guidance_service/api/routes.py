"""AI Guidance Service API routes."""

import asyncio
from datetime import UTC, datetime

from fastapi import APIRouter, HTTPException, status
from sqlalchemy import select, text

from career_guidance_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_feature_enabled,
    require_permission,
)
from career_guidance_service.db import engine
from career_guidance_service.domain.engine import build_explanation, generate_guidance
from career_guidance_service.events import publisher
from career_guidance_service.learner_scope import (
    LearnerScopeUnavailableError,
    resolve_learner_ids,
)
from career_guidance_service.models import FeatureStoreMetric, Guidance, GuidanceOutbox
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


async def _authorized_student_id(
    tenant_id: str,
    actor: CurrentActor,
    requested_student_id: str | None,
) -> str:
    if actor.tenant_id and actor.tenant_id != tenant_id:
        raise HTTPException(
            status_code=403,
            detail={"code": "tenant_mismatch", "message": "Actor tenant mismatch"},
        )
    if actor.role not in {"student", "parent", "teacher"}:
        if not requested_student_id:
            raise HTTPException(
                status_code=422,
                detail={"code": "validation_error", "message": "student_id is required"},
            )
        return requested_student_id
    if not actor.user_id:
        raise HTTPException(
            status_code=401,
            detail={"code": "unauthorized", "message": "Authenticated user is required"},
        )
    try:
        allowed = await resolve_learner_ids(tenant_id, actor.user_id, actor.role or "")
    except LearnerScopeUnavailableError as exc:
        raise HTTPException(
            status_code=503,
            detail={"code": "unavailable", "message": str(exc)},
        ) from exc
    if not requested_student_id:
        if actor.role == "student" and len(allowed) == 1:
            return next(iter(allowed))
        raise HTTPException(
            status_code=422,
            detail={"code": "validation_error", "message": "student_id is required"},
        )
    if requested_student_id not in allowed:
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    return requested_student_id


async def _authorize_guidance(
    tenant_id: str,
    actor: CurrentActor,
    guidance: Guidance,
) -> None:
    await ensure_tenant_match(tenant_id, guidance.tenant_id)
    await _authorized_student_id(tenant_id, actor, guidance.student_id)
    if actor.role in {"student", "parent"} and guidance.status != "approved":
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Guidance not found"},
        )


@public_router.get("/health", response_model=Health)
async def health_check() -> Health:
    return Health(status="ok")


async def database_ready() -> bool:
    try:
        async with asyncio.timeout(2):
            async with engine.connect() as connection:
                await connection.execute(text("SELECT 1"))
    except Exception:
        return False
    else:
        return True


@public_router.get("/ready", response_model=Health)
async def readiness_check() -> Health:
    if not await database_ready():
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail={"code": "not_ready", "message": "Database dependency is unavailable"},
        )
    return Health(status="ready")


@router.post(
    "/feature-store/metrics",
    status_code=status.HTTP_201_CREATED,
    dependencies=[require_permission("ai.view_guidance")],
)
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


@router.get(
    "/guidance",
    response_model=GuidanceList,
    dependencies=[require_permission("ai.view_guidance")],
)
async def list_guidance(
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    student_id: str | None = None,
    guidance_type: str | None = None,
) -> GuidanceList:
    resolved_student_id = await _authorized_student_id(tenant_id, actor, student_id)
    stmt = select(Guidance).where(
        Guidance.tenant_id == tenant_id,
        Guidance.student_id == resolved_student_id,
    )
    if actor.role in {"student", "parent"}:
        stmt = stmt.where(Guidance.status == "approved")
    if guidance_type:
        stmt = stmt.where(Guidance.guidance_type == guidance_type)
    stmt = stmt.order_by(Guidance.created_at.desc())
    result = await db.execute(stmt)
    items = result.scalars().all()
    return GuidanceList(
        data=[GuidanceSchema.model_validate(item) for item in items],
    )


@router.post(
    "/guidance",
    status_code=status.HTTP_201_CREATED,
    response_model=GuidanceList,
    dependencies=[require_permission("ai.approve_guidance")],
)
async def create_guidance(
    body: GenerateGuidanceRequest,
    tenant_id: TenantId,
    actor: CurrentActor,
    db: DbSession,
) -> GuidanceList:
    student_id = await _authorized_student_id(tenant_id, actor, body.student_id)
    guidance = await generate_guidance(
        db,
        tenant_id,
        student_id,
        body.guidance_types,
    )
    for item in guidance:
        db.add(item)
    await db.flush()

    if db.get_bind().dialect.name == "postgresql":
        for item in guidance:
            db.add(
                GuidanceOutbox(
                    tenant_id=tenant_id,
                    event_type="ai.guidance_generated.v1",
                    payload=publisher.guidance_event_data(item),
                )
            )
        await db.flush()
    elif publisher.publish_guidance is not None:
        await publisher.publish_guidance(tenant_id, actor.user_id, guidance)

    return GuidanceList(
        data=[GuidanceSchema.model_validate(p) for p in guidance],
    )


@router.get(
    "/guidance/{guidance_id}",
    response_model=GuidanceSchema,
    dependencies=[require_permission("ai.view_guidance")],
)
async def get_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await _authorize_guidance(tenant_id, actor, guidance)
    return GuidanceSchema.model_validate(guidance)


@router.get(
    "/guidance/{guidance_id}/explain",
    response_model=ExplainResponse,
    dependencies=[require_permission("ai.view_guidance")],
)
async def explain_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
) -> ExplainResponse:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await _authorize_guidance(tenant_id, actor, guidance)
    payload = await build_explanation(db, guidance)
    return ExplainResponse.model_validate(payload)


@router.post(
    "/guidance/{guidance_id}/approve",
    response_model=GuidanceSchema,
    dependencies=[require_permission("ai.approve_guidance")],
)
async def approve_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    _body: ReviewGuidanceRequest | None = None,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await _authorize_guidance(tenant_id, actor, guidance)
    guidance.status = "approved"
    guidance.updated_at = datetime.now(UTC)
    await db.flush()
    return GuidanceSchema.model_validate(guidance)


@router.post(
    "/guidance/{guidance_id}/reject",
    response_model=GuidanceSchema,
    dependencies=[require_permission("ai.approve_guidance")],
)
async def reject_guidance(
    guidance_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    body: ReviewGuidanceRequest | None = None,
) -> GuidanceSchema:
    result = await db.execute(select(Guidance).where(Guidance.id == guidance_id))
    guidance = result.scalar_one_or_none()
    if not guidance:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Guidance not found"},
        )
    await _authorize_guidance(tenant_id, actor, guidance)
    guidance.status = "rejected"
    if body and body.reason:
        guidance.explanation = f"Rejected: {body.reason}"
    guidance.updated_at = datetime.now(UTC)
    await db.flush()
    return GuidanceSchema.model_validate(guidance)
