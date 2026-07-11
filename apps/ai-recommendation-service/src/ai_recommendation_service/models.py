"""SQLAlchemy models for the AI Recommendation Service."""

import uuid
from datetime import UTC, datetime

from sqlalchemy import Index, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


def _now() -> datetime:
    return datetime.now(UTC)


class Base(DeclarativeBase):
    pass


class FeatureStoreMetric(Base):
    """Approved learning metrics used as model inputs."""

    __tablename__ = "feature_store_metrics"

    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(36), nullable=False, index=True)
    student_id: Mapped[str] = mapped_column(String(36), nullable=False, index=True)
    metric_key: Mapped[str] = mapped_column(String(128), nullable=False)
    value: Mapped[float] = mapped_column(nullable=False)
    source: Mapped[str] = mapped_column(String(32), nullable=False)
    recorded_at: Mapped[datetime] = mapped_column(nullable=False)
    created_at: Mapped[datetime] = mapped_column(default=_now)

    __table_args__ = (
        Index("ix_feature_store_metrics_tenant_student", "tenant_id", "student_id"),
    )


class Recommendation(Base):
    """AI-generated learning recommendation with teacher approval workflow."""

    __tablename__ = "recommendations"

    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=lambda: str(uuid.uuid4()))
    tenant_id: Mapped[str] = mapped_column(String(36), nullable=False, index=True)
    student_id: Mapped[str] = mapped_column(String(36), nullable=False, index=True)
    recommendation_type: Mapped[str] = mapped_column(String(64), nullable=False)
    title: Mapped[str] = mapped_column(String(255), nullable=False)
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    status: Mapped[str] = mapped_column(String(16), default="pending")
    confidence: Mapped[float] = mapped_column(default=0.0)
    explanation: Mapped[str | None] = mapped_column(Text, nullable=True)
    approved_by: Mapped[str | None] = mapped_column(String(36), nullable=True)
    approved_at: Mapped[datetime | None] = mapped_column(nullable=True)
    created_at: Mapped[datetime] = mapped_column(default=_now)
    updated_at: Mapped[datetime] = mapped_column(default=_now, onupdate=_now)

    __table_args__ = (
        Index("ix_recommendations_tenant_student_status", "tenant_id", "student_id", "status"),
    )
