# AI Recommendation Service

AuraEDU AI Recommendation Service (EP-30). FastAPI service that generates
explainable learning recommendations, supports teacher approval/override, and
feeds from an internal feature store.

PostgreSQL startup applies numbered migrations under `migrations/` while holding
the shared AI-schema advisory lock. Every request and subscribed event binds its
transaction to the canonical tenant code; forced row-level-security policies
protect both recommendations and shared feature-store metrics.

Recommendation creation and its `ai.recommendation_generated.v1` integration
event commit in one PostgreSQL transaction. A background transactional-outbox
dispatcher publishes the stable outbox ID as both the CloudEvent ID and NATS
deduplication ID, then retries failures with capped exponential backoff. The
SQLite test adapter retains a direct in-memory publisher only for local tests.

## Local development

```bash
uv sync
uv run pytest
uv run uvicorn ai_recommendation_service.main:app --reload --port 8200
```

## API

Base path (after gateway): `/api/v1/ai/recommendations`

- `GET /health`
- `POST /feature-store/metrics`
- `GET /recommendations?student_id=...&status=...`
- `POST /recommendations` — generate recommendations for a student
- `GET /recommendations/{id}`
- `POST /recommendations/{id}/approve`
- `POST /recommendations/{id}/reject`
- `POST /recommendations/{id}/override`
- `GET /recommendations/{id}/explain`
