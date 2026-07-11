"""FastAPI entrypoint for the AI Recommendation Service."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.responses import JSONResponse

from ai_recommendation_service.api.routes import router
from ai_recommendation_service.db import engine
from ai_recommendation_service.events.publisher import RecommendationPublisher
from ai_recommendation_service.events.subscriber import FeatureStoreSubscriber
from ai_recommendation_service.models import Base

app_publisher = RecommendationPublisher()
app_subscriber = FeatureStoreSubscriber()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None]:  # noqa: ARG001
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    await app_publisher.connect()
    await app_subscriber.connect()
    await app_subscriber.start()
    yield
    await app_subscriber.close()
    await app_publisher.close()
    await engine.dispose()


app = FastAPI(
    title="AuraEDU AI Recommendation Service",
    version="1.0.0",
    lifespan=lifespan,
)


@app.exception_handler(ValueError)
async def value_error_handler(request: object, exc: ValueError) -> JSONResponse:  # noqa: ARG001
    return JSONResponse(
        status_code=422,
        content={"code": "validation_error", "message": str(exc)},
    )


app.include_router(router)
