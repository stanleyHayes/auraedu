import uuid
from datetime import UTC, datetime

from sqlalchemy import JSON, DateTime, Float, Index, Integer, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


def _now() -> datetime:
    return datetime.now(UTC)


class FeatureStoreMetric(Base):
    __tablename__ = "feature_store_metrics"
    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(64))
    student_id: Mapped[str] = mapped_column(String(36))
    metric_key: Mapped[str] = mapped_column(String(128))
    value: Mapped[float]
    source: Mapped[str] = mapped_column(String(32))
    recorded_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    __table_args__ = (Index("ix_feature_store_metrics_tenant_student", "tenant_id", "student_id"),)


class Prediction(Base):
    __tablename__ = "predictions"
    id: Mapped[str] = mapped_column(primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(64))
    student_id: Mapped[str] = mapped_column(String(36))
    prediction_type: Mapped[str] = mapped_column(String(50))
    title: Mapped[str] = mapped_column(String(200), default="")
    value: Mapped[float] = mapped_column(Float, default=0.0)
    confidence: Mapped[float]
    status: Mapped[str] = mapped_column(String(20), default="pending")
    explanation: Mapped[str] = mapped_column(Text)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, onupdate=_now
    )
    __table_args__ = (Index("idx_prediction_student", "tenant_id", "student_id"),)


class PredictionOutbox(Base):
    __tablename__ = "prediction_outbox"
    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(64), nullable=False)
    event_type: Mapped[str] = mapped_column(String(128), nullable=False)
    payload: Mapped[dict[str, object]] = mapped_column(JSON, nullable=False)
    attempts: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    next_attempt_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, nullable=False
    )
    last_error: Mapped[str | None] = mapped_column(Text, nullable=True)
    published_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, nullable=False
    )
