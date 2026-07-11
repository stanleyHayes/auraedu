# AI Recommendation Service

AuraEDU AI Recommendation Service (EP-30). FastAPI service that generates
explainable learning recommendations, supports teacher approval/override, and
feeds from an internal feature store.

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
