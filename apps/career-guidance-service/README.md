# Career Guidance Service

AuraEDU Career Guidance Service (EP-32). FastAPI service that generates
career-path and course-load guidance from an internal feature store.

PostgreSQL startup applies numbered migrations under `migrations/` while holding
the shared AI-schema advisory lock. Every request and subscribed event binds its
transaction to the canonical tenant code; forced row-level-security policies
protect both guidance and shared feature-store metrics.

Guidance creation and its `ai.guidance_generated.v1` integration event commit
in one PostgreSQL transaction. The embedded transactional-outbox dispatcher
publishes stable CloudEvent and NATS deduplication IDs and retries broker
failures with capped exponential backoff. It never treats the local console
transport as successful durable delivery.

## Local development

```bash
uv sync
uv run pytest
uv run uvicorn career_guidance_service.main:app --reload --port 8112
```

## API

Base path (after gateway): `/api/v1/ai/career-guidance`

- `GET /health`
- `POST /feature-store/metrics`
- `GET /guidance?student_id=...&guidance_type=...`
- `POST /guidance` — generate guidance for a student
- `GET /guidance/{guidance_id}`
- `GET /guidance/{guidance_id}/explain`
- `POST /guidance/{guidance_id}/approve`
- `POST /guidance/{guidance_id}/reject`
