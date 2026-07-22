"""Service configuration."""

import os
import re

from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class ProductionConfigError(RuntimeError):
    """A required production binding is missing or unsafe."""

    def __init__(self, key: str, reason: str) -> None:
        super().__init__(f"{key} {reason}")


class Settings(BaseSettings):
    """Settings loaded from environment variables."""

    model_config = SettingsConfigDict(env_prefix="AI_REC_")

    port: int = 8200
    database_url: str = "postgresql+asyncpg://auraedu:auraedu@localhost:5432/ai"
    nats_host: str = "nats://localhost:4222"
    service_name: str = "ai-recommendation-service"
    debug: bool = False
    student_service_url: str = "http://student-service:8083"
    internal_service_token: str = ""

    @field_validator("database_url")
    @classmethod
    def use_async_postgres_driver(cls, value: str) -> str:
        """Render supplies a standard PostgreSQL URL; SQLAlchemy needs asyncpg."""
        if value.startswith("postgresql://"):
            value = value.replace("postgresql://", "postgresql+asyncpg://", 1)
        elif value.startswith("postgres://"):
            value = value.replace("postgres://", "postgresql+asyncpg://", 1)
        return re.sub(r"([?&])sslmode=", r"\1ssl=", value)


settings = Settings()


def postgres_dsn(value: str) -> str:
    """Return the libpq DSN expected by direct asyncpg migration connections."""
    value = value.replace("postgresql+asyncpg://", "postgresql://", 1)
    return re.sub(r"([?&])ssl=", r"\1sslmode=", value)


def validate_production_runtime(current: Settings = settings) -> None:
    """Reject local defaults or missing private credentials in production."""
    if os.getenv("ENVIRONMENT", "development").strip().lower() != "production":
        return
    values = {
        "AI_REC_DATABASE_URL": current.database_url,
        "AI_REC_NATS_HOST": current.nats_host,
        "AI_REC_STUDENT_SERVICE_URL": current.student_service_url,
        "AI_REC_TENANT_SERVICE_URL": os.getenv("AI_REC_TENANT_SERVICE_URL", ""),
        "INTERNAL_SERVICE_TOKEN": current.internal_service_token
        or os.getenv("INTERNAL_SERVICE_TOKEN", ""),
    }
    for key, value in values.items():
        normalized = value.strip().lower()
        if not normalized:
            raise ProductionConfigError(key, "is required in production")
        if key.endswith("DATABASE_URL") and not normalized.startswith("postgresql+asyncpg://"):
            raise ProductionConfigError(key, "must use PostgreSQL in production")
        if "localhost" in normalized or "127.0.0.1" in normalized:
            raise ProductionConfigError(key, "cannot use a local endpoint in production")
    if current.debug:
        raise ProductionConfigError("AI_REC_DEBUG", "must be disabled in production")


def nats_url(value: str) -> str:
    """Normalize Render's scheme-less private host into a NATS client URL."""
    host = value.strip()
    if not host or host.startswith(("nats://", "tls://")):
        return host
    return f"nats://{host}"
