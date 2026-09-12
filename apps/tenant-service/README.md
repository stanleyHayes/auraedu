# Tenant service

## Selectable persistence (AURA-9.12)

Server, worker and `migrate` default to `DATABASE_DRIVER=postgres`. Select
`mongodb` with `MONGODB_URI` pointing at a replica set (including Atlas).
Startup verifies transaction support before readiness. `MONGODB_DATABASE`
defaults to `auraedu_tenant`; `MONGODB_MAX_POOL_SIZE` defaults to 2.
Invalid driver/configuration values fail closed. `BIND_HOST` optionally limits
the HTTP listener. PostgreSQL migrations remain available.

Mongo scopes every tenant operation and uses transactions for multi-document
changes, with stable embedded outbox IDs for at-least-once delivery.
Run conformance and adversarial tests with:
`TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1 -run TestMongo ./internal/adapters/...`.
