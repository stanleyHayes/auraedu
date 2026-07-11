# AI Prediction Service

AuraEDU AI Prediction Service (EP-31). FastAPI service that generates
explainable risk and performance predictions from an internal feature store.

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
