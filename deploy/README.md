# Deploy — local orchestration

This directory holds the local Docker Compose orchestration for AuraEDU.
Production runs on **Render** via the root [`render.yaml`](../render.yaml) Blueprint.

| File | Purpose | Command |
|---|---|---|
| `docker-compose.infra.yml` | Postgres 17, Valkey, NATS JetStream, OTel collector | `make infra-up` / `make infra-down` |
| `docker-compose.yml` | Full local stack (infra + Go services + Next.js frontends) | `make dev` / `make dev-down` |
| `postgres-init.sql` | Creates one logical database per service on first Postgres startup |

## Notes

- `docker-compose.yml` includes `docker-compose.infra.yml`, so `make dev` starts everything.
- Every service connects only to its own logical DB (DB-per-service, `agent_plan.md` §3/§5.2).
- Cloudinary is used for media even in local dev; there is no MinIO/S3 fallback.
- The OTel collector currently logs telemetry to the console. Full Grafana/Loki/Tempo
  dashboards land in Sprint 6 (EP-08).
- Validate compose files with `make compose-validate`.
