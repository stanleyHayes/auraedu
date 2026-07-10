# website-service

Per-school public site content/pages (EP-19, L2).

Implements the Website Service minimal CRUD for `Page` and `Section` aggregates:
- Domain model with validation and patch `ApplyUpdate`
- Postgres adapter with cursor pagination, tenant-scoped filters, and RLS
- HTTP adapter for pages and sections
- Event publishing over `platform/eventbus`
- Feature-flag gating (`public_website`) and RBAC (`website.read`, `website.manage`)
- Worker that provisions a default `home` page with a hero section on `tenant.created.v1`

## Run

```bash
GOFLAGS=-mod=readonly go run ./cmd/server   # from apps/website-service
curl localhost:8080/health
```

## Contract

REST: `contracts/openapi/website.v1.yaml` · Events: `contracts/events/`.
Every action enforces: authenticated → tenant → RBAC → feature-flag → ownership.
