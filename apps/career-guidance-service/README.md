# Career Guidance Service

AuraEDU Career Guidance Service (EP-32). FastAPI service that generates
career-path and course-load guidance from an internal feature store.

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
