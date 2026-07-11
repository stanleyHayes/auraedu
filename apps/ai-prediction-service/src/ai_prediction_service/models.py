import uuid
from datetime import UTC, datetime

from sqlalchemy import Float, Index, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


def _now() -> datetime:
    return datetime.now(UTC)


class FeatureStoreMetric(Base):
    __tablename__ = "feature_store_metrics"
    id: Mapped[str] = mapped_column(primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(36))
    student_id: Mapped[str] = mapped_column(String(36))
    metric_key: Mapped[str] = mapped_column(String(100))
    value: Mapped[float]
    source: Mapped[str] = mapped_column(String(100))
    recorded_at: Mapped[datetime] = mapped_column(default=_now)
    created_at: Mapped[datetime] = mapped_column(default=_now)
    __table_args__ = (Index("idx_metric_student", "tenant_id", "student_id", "metric_key"),)


class Prediction(Base):
    __tablename__ = "predictions"
    id: Mapped[str] = mapped_column(primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(36))
    student_id: Mapped[str] = mapped_column(String(36))
    prediction_type: Mapped[str] = mapped_column(String(50))
    title: Mapped[str] = mapped_column(String(200), default="")
    value: Mapped[float] = mapped_column(Float, default=0.0)
    confidence: Mapped[float]
    status: Mapped[str] = mapped_column(String(20), default="pending")
    explanation: Mapped[str] = mapped_column(Text)
    created_at: Mapped[datetime] = mapped_column(default=_now)
    updated_at: Mapped[datetime] = mapped_column(default=_now, onupdate=_now)
    __table_args__ = (Index("idx_prediction_student", "tenant_id", "student_id"),)
