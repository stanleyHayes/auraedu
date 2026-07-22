"""AI Prediction Service API routes."""

import asyncio
from datetime import UTC, datetime

from fastapi import APIRouter, HTTPException, status
from sqlalchemy import select, text

from ai_prediction_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_feature_enabled,
    require_permission,
)
from ai_prediction_service.db import engine
from ai_prediction_service.domain.engine import build_explanation, generate_predictions
from ai_prediction_service.events import publisher
from ai_prediction_service.learner_scope import (
    LearnerScopeUnavailableError,
    resolve_learner_ids,
)
from ai_prediction_service.models import FeatureStoreMetric, Prediction, PredictionOutbox
from ai_prediction_service.schemas import (
    ExplainResponse,
    FeatureStoreMetricSchema,
    GeneratePredictionsRequest,
    Health,
    IngestMetricRequest,
    PredictionList,
    ReviewPredictionRequest,
)
from ai_prediction_service.schemas import (
    Prediction as PredictionSchema,
)

public_router = APIRouter(tags=["predictions"])
router = APIRouter(tags=["predictions"], dependencies=[require_feature_enabled("ai_predictions")])


async def _authorize_student(tenant_id: str, actor: CurrentActor, student_id: str) -> None:
    if actor.tenant_id and actor.tenant_id != tenant_id:
        raise HTTPException(
            status_code=403,
            detail={"code": "tenant_mismatch", "message": "Actor tenant mismatch"},
        )
    if actor.role not in {"student", "parent", "teacher"}:
        return
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
    if student_id not in allowed:
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Prediction not found"},
        )


async def _authorize_prediction(
    tenant_id: str,
    actor: CurrentActor,
    prediction: Prediction,
) -> None:
    await ensure_tenant_match(tenant_id, prediction.tenant_id)
    await _authorize_student(tenant_id, actor, prediction.student_id)
    if actor.role in {"student", "parent"} and prediction.status != "approved":
        raise HTTPException(
            status_code=404,
            detail={"code": "not_found", "message": "Prediction not found"},
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
    dependencies=[require_permission("ai.view_predictions")],
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
    "/predictions",
    response_model=PredictionList,
    dependencies=[require_permission("ai.view_predictions")],
)
async def list_predictions(
    tenant_id: TenantId,
    student_id: str,
    db: DbSession,
    actor: CurrentActor,
    prediction_type: str | None = None,
) -> PredictionList:
    await _authorize_student(tenant_id, actor, student_id)
    stmt = select(Prediction).where(
        Prediction.tenant_id == tenant_id,
        Prediction.student_id == student_id,
    )
    if prediction_type:
        stmt = stmt.where(Prediction.prediction_type == prediction_type)
    if actor.role in {"student", "parent"}:
        stmt = stmt.where(Prediction.status == "approved")
    stmt = stmt.order_by(Prediction.created_at.desc())
    result = await db.execute(stmt)
    items = result.scalars().all()
    return PredictionList(
        data=[PredictionSchema.model_validate(item) for item in items],
    )


@router.post(
    "/predictions",
    status_code=status.HTTP_201_CREATED,
    response_model=PredictionList,
    dependencies=[require_permission("ai.approve_predictions")],
)
async def create_predictions(
    body: GeneratePredictionsRequest,
    tenant_id: TenantId,
    actor: CurrentActor,
    db: DbSession,
) -> PredictionList:
    await _authorize_student(tenant_id, actor, body.student_id)
    predictions = await generate_predictions(
        db,
        tenant_id,
        body.student_id,
        body.prediction_types,
    )
    for prediction in predictions:
        db.add(prediction)
    await db.flush()

    if db.get_bind().dialect.name == "postgresql":
        for prediction in predictions:
            db.add(
                PredictionOutbox(
                    tenant_id=tenant_id,
                    event_type="ai.prediction_generated.v1",
                    payload=publisher.prediction_event_data(prediction),
                )
            )
        await db.flush()
    elif publisher.publish_predictions is not None:
        await publisher.publish_predictions(tenant_id, actor.user_id, predictions)

    return PredictionList(
        data=[PredictionSchema.model_validate(p) for p in predictions],
    )


@router.get(
    "/predictions/{prediction_id}",
    response_model=PredictionSchema,
    dependencies=[require_permission("ai.view_predictions")],
)
async def get_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
) -> PredictionSchema:
    result = await db.execute(select(Prediction).where(Prediction.id == prediction_id))
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await _authorize_prediction(tenant_id, actor, prediction)
    return PredictionSchema.model_validate(prediction)


@router.get(
    "/predictions/{prediction_id}/explain",
    response_model=ExplainResponse,
    dependencies=[require_permission("ai.view_predictions")],
)
async def explain_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
) -> ExplainResponse:
    result = await db.execute(select(Prediction).where(Prediction.id == prediction_id))
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await _authorize_prediction(tenant_id, actor, prediction)
    payload = await build_explanation(db, prediction)
    return ExplainResponse.model_validate(payload)


@router.post(
    "/predictions/{prediction_id}/approve",
    response_model=PredictionSchema,
    dependencies=[require_permission("ai.approve_predictions")],
)
async def approve_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    _body: ReviewPredictionRequest | None = None,
) -> PredictionSchema:
    result = await db.execute(select(Prediction).where(Prediction.id == prediction_id))
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await _authorize_prediction(tenant_id, actor, prediction)
    prediction.status = "approved"
    prediction.updated_at = datetime.now(UTC)
    await db.flush()
    return PredictionSchema.model_validate(prediction)


@router.post(
    "/predictions/{prediction_id}/reject",
    response_model=PredictionSchema,
    dependencies=[require_permission("ai.approve_predictions")],
)
async def reject_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    actor: CurrentActor,
    body: ReviewPredictionRequest | None = None,
) -> PredictionSchema:
    result = await db.execute(select(Prediction).where(Prediction.id == prediction_id))
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await _authorize_prediction(tenant_id, actor, prediction)
    prediction.status = "rejected"
    if body and body.reason:
        prediction.explanation = f"Rejected: {body.reason}"
    prediction.updated_at = datetime.now(UTC)
    await db.flush()
    return PredictionSchema.model_validate(prediction)
