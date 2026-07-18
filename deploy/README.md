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
- The stack covers every service in `agent_plan.md` Appendix D: all 18 Go/Python services,
  the API gateway, the two frontends, and the event workers (`report`, `notification`,
  `analytics`, `website`, `billing`, `audit`).
- Every service connects only to its own logical DB (DB-per-service, `agent_plan.md` §3/§5.2).
- Go services run goose migrations on boot. The images don't ship migration files, so each
  service mounts `apps/<svc>/migrations` at `/migrations` (CWD is `/`); first boot migrates
  every DB, no manual step needed. `docker compose run --rm <svc> migrate` also works.
- Container port == host port == Appendix D port (set via `PORT`), so the gateway's
  `SERVICE_<NAME>_URL` values work identically inside and outside Docker.
- Feature flags load from `contracts/features/features.yaml` (mounted at `/contracts` for
  services whose image doesn't bake it in); without it a service boots but all its
  feature-gated routes return `feature_disabled`.
- Cloudinary is used for media even in local dev; there is no MinIO/S3 fallback.
- The OTel collector currently logs telemetry to the console. Full Grafana/Loki/Tempo
  dashboards land in Sprint 6 (EP-08).
- Validate compose files with `make compose-validate`.
