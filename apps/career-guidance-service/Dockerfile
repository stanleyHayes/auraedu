FROM ghcr.io/astral-sh/uv:python3.14-bookworm-slim@sha256:7cf77f594be8042dab6daa9fe326f90962252268b4f120a7f5dccce4d947e6c1 AS builder

ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy
ENV UV_PYTHON_DOWNLOADS=0

WORKDIR /app

# Copy workspace definition and lockfile first for layer caching.
COPY pyproject.toml uv.lock ./
COPY apps/ai-recommendation-service/ ./apps/ai-recommendation-service/
COPY apps/ai-prediction-service/ ./apps/ai-prediction-service/
COPY apps/career-guidance-service/ ./apps/career-guidance-service/

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev --package auraedu-career-guidance-service

FROM python:3.14-slim-bookworm@sha256:86f975aca15cf04a40b399eebede9aea7c82eae084d1f1a0a6ef6bcaae871a30 AS runner

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PATH="/app/.venv/bin:$PATH"

COPY --chown=65532:65532 --from=builder /app/.venv ./.venv
COPY --chown=65532:65532 --from=builder /app/apps/career-guidance-service ./apps/career-guidance-service
COPY --chown=65532:65532 contracts/features/features.yaml ./contracts/features/features.yaml

EXPOSE 8112

USER 65532:65532

CMD ["uvicorn", "career_guidance_service.main:app", "--host", "0.0.0.0", "--port", "8112"]
