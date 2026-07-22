"""FastAPI entrypoint for the Career Guidance Service."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse

from career_guidance_service.api.routes import public_router, router
from career_guidance_service.config import validate_production_runtime
from career_guidance_service.db import engine, initialize_database
from career_guidance_service.events.outbox import OutboxDispatcher
from career_guidance_service.events.subscriber import FeatureStoreSubscriber
from career_guidance_service.events.transport import close_transport
from career_guidance_service.models import Base
from career_guidance_service.observability import PrometheusMiddleware, RequestBodyLimitMiddleware

app_subscriber = FeatureStoreSubscriber()
outbox_dispatcher = OutboxDispatcher()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None]:  # noqa: ARG001
    validate_production_runtime()
    await initialize_database(Base.metadata)
    await app_subscriber.connect()
    await app_subscriber.start()
    await outbox_dispatcher.start()
    yield
    await outbox_dispatcher.close()
    await app_subscriber.close()
    await close_transport()
    await engine.dispose()


app = FastAPI(
    title="AuraEDU Career Guidance Service",
    version="1.0.0",
    lifespan=lifespan,
)
app.add_middleware(RequestBodyLimitMiddleware)
app.add_middleware(PrometheusMiddleware, service="career-guidance-service")


@app.exception_handler(HTTPException)
async def http_exception_handler(request: object, exc: HTTPException) -> JSONResponse:  # noqa: ARG001
    content = exc.detail if isinstance(exc.detail, dict) else {"message": exc.detail}
    return JSONResponse(status_code=exc.status_code, content=content)


@app.exception_handler(ValueError)
async def value_error_handler(request: object, exc: ValueError) -> JSONResponse:  # noqa: ARG001
    return JSONResponse(
        status_code=422,
        content={"code": "validation_error", "message": str(exc)},
    )


app.include_router(public_router)
app.include_router(router)
# The gateway preserves the full public contract path when proxying.
app.include_router(router, prefix="/api/v1/ai/career-guidance")
