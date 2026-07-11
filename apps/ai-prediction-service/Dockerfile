FROM ghcr.io/astral-sh/uv:python3.14-bookworm-slim AS builder

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy
ENV UV_PYTHON_DOWNLOADS=0

WORKDIR /app

# Copy workspace definition and lockfile first for layer caching.
COPY pyproject.toml uv.lock ./
COPY apps/ai-recommendation-service/ ./apps/ai-recommendation-service/
COPY apps/ai-prediction-service/ ./apps/ai-prediction-service/
COPY apps/career-guidance-service/ ./apps/career-guidance-service/

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev --package auraedu-ai-prediction-service

FROM python:3.14-slim-bookworm AS runner

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PATH="/app/.venv/bin:$PATH"

COPY --from=builder /app/.venv ./.venv
COPY --from=builder /app/apps/ai-prediction-service ./apps/ai-prediction-service
COPY contracts/features/features.yaml ./contracts/features/features.yaml

EXPOSE 8201

CMD ["uvicorn", "ai_prediction_service.main:app", "--host", "0.0.0.0", "--port", "8201"]
