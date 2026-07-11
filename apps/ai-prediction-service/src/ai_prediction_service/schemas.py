"""Request/response schemas aligned with the OpenAPI contract."""

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class Health(BaseModel):
    status: str


class IngestMetricRequest(BaseModel):
    student_id: str
    metric_key: str
    value: float
    source: Literal["assessment", "attendance", "analytics"]
    recorded_at: datetime


class FeatureStoreMetricSchema(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    tenant_id: str
    student_id: str
    metric_key: str
    value: float
    source: str
    recorded_at: datetime
    created_at: datetime


class GeneratePredictionsRequest(BaseModel):
    student_id: str
    prediction_types: list[str] | None = None


class ExplainFactor(BaseModel):
    metric_key: str
    value: float
    contribution: Literal[
        "strong_positive",
        "positive",
        "neutral",
        "negative",
        "strong_negative",
    ] = "neutral"


class Prediction(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    tenant_id: str
    student_id: str
    prediction_type: str
    title: str
    value: float
    confidence: float = Field(..., ge=0.0, le=1.0)
    status: Literal["pending", "approved", "rejected"] = "pending"
    explanation: str | None = None
    created_at: datetime
    updated_at: datetime


class PredictionList(BaseModel):
    data: list[Prediction]
    next_cursor: str | None = None


class ExplainResponse(BaseModel):
    prediction_id: str
    factors: list[ExplainFactor]
    model_notes: str | None = None


class ReviewPredictionRequest(BaseModel):
    reason: str | None = None
