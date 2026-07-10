# api-gateway

The single public entry point for AuraEDU (agent_plan §7). Owns routing to services,
JWT auth verification, tenant resolution, rate limiting, request-id, CORS, API versioning.

**Render type:** `web` (public). **Lane:** L1.

## Status
- ✅ Sprint 0: liveness `/health` + readiness `/ready`, graceful shutdown, structured logs.
- ✅ Sprint 1 (EP-03):
  - AURA-3.1: routing + service registry for `/api/v1/*`
  - AURA-3.2: JWT verification via `platform/auth`
  - AURA-3.3: tenant resolution (header/subdomain/Tenant Service stub) with rejection
  - AURA-3.4: Redis/Valkey token-bucket rate limiting, request-id, CORS, structured access logs
  - AURA-3.5: feature-flag edge pre-check (403 `feature_disabled`)

## Run locally
```bash
JWT_SIGNING_KEY=dev-signing-key REDIS_URL=redis://localhost:6379 \
  GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/api-gateway
curl localhost:8080/health
```

## Layout
Hexagonal (agent_plan §5): `cmd/server`, `internal/{gateway,stubs,mocks}`.
Local stubs for `platform/tenancy` and `platform/flags` are used until those
packages land; they are not shared outside the gateway.
