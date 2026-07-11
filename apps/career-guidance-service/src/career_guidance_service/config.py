"""Service configuration."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Settings loaded from environment variables."""

    model_config = SettingsConfigDict(env_prefix="AI_GUIDANCE_")

    port: int = 8112
    database_url: str = "postgresql+asyncpg://auraedu:auraedu@localhost:5432/ai"
    nats_host: str = "nats://localhost:4222"
    service_name: str = "career-guidance-service"
    debug: bool = False


settings = Settings()
