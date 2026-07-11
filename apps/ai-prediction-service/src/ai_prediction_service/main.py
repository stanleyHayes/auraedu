"""FastAPI entrypoint for the AI Prediction Service."""

from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse

from ai_prediction_service.api.routes import public_router, router
from ai_prediction_service.db import engine
from ai_prediction_service.events.subscriber import FeatureStoreSubscriber
from ai_prediction_service.events.transport import close_transport
from ai_prediction_service.models import Base

app_subscriber = FeatureStoreSubscriber()


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None]:  # noqa: ARG001
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    await app_subscriber.connect()
    await app_subscriber.start()
    yield
    await app_subscriber.close()
    await close_transport()
    await engine.dispose()


app = FastAPI(
    title="AuraEDU AI Prediction Service",
    version="1.0.0",
    lifespan=lifespan,
)


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
