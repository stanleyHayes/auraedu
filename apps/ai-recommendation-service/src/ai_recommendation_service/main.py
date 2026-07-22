"""FastAPI entrypoint for the AI Recommendation Service."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse

from ai_recommendation_service.api.routes import public_router, router
from ai_recommendation_service.config import validate_production_runtime
from ai_recommendation_service.db import engine, initialize_database
from ai_recommendation_service.events.publisher import RecommendationPublisher
from ai_recommendation_service.events.subscriber import FeatureStoreSubscriber
from ai_recommendation_service.models import Base
from ai_recommendation_service.observability import (
    PrometheusMiddleware,
    RequestBodyLimitMiddleware,
)

app_publisher = RecommendationPublisher()
app_subscriber = FeatureStoreSubscriber()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None]:  # noqa: ARG001
    validate_production_runtime()
    await initialize_database(Base.metadata)
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
app.add_middleware(RequestBodyLimitMiddleware)
app.add_middleware(PrometheusMiddleware, service="ai-recommendation-service")


@app.exception_handler(ValueError)
async def value_error_handler(request: object, exc: ValueError) -> JSONResponse:  # noqa: ARG001
    return JSONResponse(
        status_code=422,
        content={"code": "validation_error", "message": str(exc)},
    )


@app.exception_handler(HTTPException)
async def http_exception_handler(request: object, exc: HTTPException) -> JSONResponse:  # noqa: ARG001
    content = exc.detail if isinstance(exc.detail, dict) else {"message": exc.detail}
    return JSONResponse(status_code=exc.status_code, content=content)


app.include_router(public_router)
app.include_router(router)
# The gateway preserves the public contract path. Keep the root routes for
# service-local probes and backwards-compatible internal callers.
app.include_router(router, prefix="/api/v1/ai")
