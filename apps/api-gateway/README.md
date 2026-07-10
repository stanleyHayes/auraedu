# api-gateway

The single public entry point for AuraEDU (agent_plan §7). Owns routing to services,
JWT auth verification, tenant resolution, rate limiting, request-id, CORS, API versioning.

**Render type:** `web` (public). **Lane:** L1.

## Status
- ✅ Sprint 0: liveness `/health` + readiness `/ready`, graceful shutdown, structured logs.
- ⏳ Sprint 1 (EP-03): routing + service registry (AURA-3.1), auth verify (3.2), tenant
  resolution + reject-without-tenant (3.3), rate limit/request-id/CORS (3.4), feature
  pre-check (3.5).

## Run locally
```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/api-gateway
# or from repo root:
make dev
curl localhost:8080/health
```

## Layout
Hexagonal (agent_plan §5): `cmd/server`, `cmd/worker`, `internal/{domain,application,ports,adapters}`.
Business logic never lives in `adapters/http`.
