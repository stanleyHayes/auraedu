# AI Prediction Service

AuraEDU AI Prediction Service (EP-31). FastAPI service that generates
explainable risk and performance predictions from an internal feature store.

PostgreSQL startup applies numbered migrations under `migrations/` while holding
the shared AI-schema advisory lock. Every request and subscribed event binds its
transaction to the canonical tenant code; forced row-level-security policies
protect both predictions and shared feature-store metrics.

Prediction creation and its `ai.prediction_generated.v1` integration event
commit in one PostgreSQL transaction. The embedded transactional-outbox
dispatcher publishes stable CloudEvent and NATS deduplication IDs and retries
broker failures with capped exponential backoff. It never treats the local
console transport as successful durable delivery.

## Local development

```bash
uv sync
uv run pytest
uv run uvicorn ai_prediction_service.main:app --reload --port 8201
```

## API

Base path (after gateway): `/api/v1/ai/predictions`

- `GET /health`
- `POST /feature-store/metrics`
- `GET /predictions?student_id=...&type=...`
- `POST /predictions` — generate predictions for a student
- `GET /predictions/{prediction_id}`
- `GET /predictions/{prediction_id}/explain`
