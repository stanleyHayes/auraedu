"""AI Prediction Service API routes."""

from datetime import UTC, datetime

from fastapi import APIRouter, HTTPException, status
from sqlalchemy import select

from ai_prediction_service.api.dependencies import (
    CurrentActor,
    DbSession,
    TenantId,
    ensure_tenant_match,
    require_feature_enabled,
)
from ai_prediction_service.domain.engine import build_explanation, generate_predictions
from ai_prediction_service.events import publisher
from ai_prediction_service.models import FeatureStoreMetric, Prediction
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


@router.get("/predictions", response_model=PredictionList)
async def list_predictions(
    tenant_id: TenantId,
    student_id: str,
    db: DbSession,
    prediction_type: str | None = None,
) -> PredictionList:
    stmt = select(Prediction).where(
        Prediction.tenant_id == tenant_id,
        Prediction.student_id == student_id,
    )
    if prediction_type:
        stmt = stmt.where(Prediction.prediction_type == prediction_type)
    stmt = stmt.order_by(Prediction.created_at.desc())
    result = await db.execute(stmt)
    items = result.scalars().all()
    return PredictionList(
        data=[PredictionSchema.model_validate(item) for item in items],
    )


@router.post("/predictions", status_code=status.HTTP_201_CREATED, response_model=PredictionList)
async def create_predictions(
    body: GeneratePredictionsRequest,
    tenant_id: TenantId,
    actor: CurrentActor,
    db: DbSession,
) -> PredictionList:
    predictions = await generate_predictions(
        db,
        tenant_id,
        body.student_id,
        body.prediction_types,
    )
    for prediction in predictions:
        db.add(prediction)
    await db.flush()

    if publisher.publish_predictions is not None:
        await publisher.publish_predictions(tenant_id, actor.user_id, predictions)

    return PredictionList(
        data=[PredictionSchema.model_validate(p) for p in predictions],
    )


@router.get("/predictions/{prediction_id}", response_model=PredictionSchema)
async def get_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
) -> PredictionSchema:
    result = await db.execute(
        select(Prediction).where(Prediction.id == prediction_id)
    )
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await ensure_tenant_match(tenant_id, prediction.tenant_id)
    return PredictionSchema.model_validate(prediction)


@router.get("/predictions/{prediction_id}/explain", response_model=ExplainResponse)
async def explain_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
) -> ExplainResponse:
    result = await db.execute(
        select(Prediction).where(Prediction.id == prediction_id)
    )
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await ensure_tenant_match(tenant_id, prediction.tenant_id)
    payload = await build_explanation(db, prediction)
    return ExplainResponse.model_validate(payload)


@router.post("/predictions/{prediction_id}/approve", response_model=PredictionSchema)
async def approve_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    body: ReviewPredictionRequest | None = None,
) -> PredictionSchema:
    result = await db.execute(
        select(Prediction).where(Prediction.id == prediction_id)
    )
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await ensure_tenant_match(tenant_id, prediction.tenant_id)
    prediction.status = "approved"
    prediction.updated_at = datetime.now(UTC)
    await db.flush()
    return PredictionSchema.model_validate(prediction)


@router.post("/predictions/{prediction_id}/reject", response_model=PredictionSchema)
async def reject_prediction(
    prediction_id: str,
    tenant_id: TenantId,
    db: DbSession,
    body: ReviewPredictionRequest | None = None,
) -> PredictionSchema:
    result = await db.execute(
        select(Prediction).where(Prediction.id == prediction_id)
    )
    prediction = result.scalar_one_or_none()
    if not prediction:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail={"code": "not_found", "message": "Prediction not found"},
        )
    await ensure_tenant_match(tenant_id, prediction.tenant_id)
    prediction.status = "rejected"
    if body and body.reason:
        prediction.explanation = f"Rejected: {body.reason}"
    prediction.updated_at = datetime.now(UTC)
    await db.flush()
    return PredictionSchema.model_validate(prediction)
