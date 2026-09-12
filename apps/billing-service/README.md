# billing-service

SaaS plans, tenant subscriptions, trials and platform invoices (EP-22, L2).

Trial creation, subscription plan changes/upgrades and invoice creation commit their
promised CloudEvents atomically through a FORCE-RLS transactional outbox. The deployed
`billing-service worker` consumes tenant onboarding and publishes pending outbox records
to JetStream with stable IDs and bounded retries.

Subscription/invoice updates, payment-state changes and deletes are intentionally
non-event boundaries until versioned integration contracts are introduced for them.

## Selectable persistence (AURA-9.12)

Both server and worker select `DATABASE_DRIVER=postgres` (the default) or
`DATABASE_DRIVER=mongodb`. PostgreSQL retains its migrations and `DATABASE_URL`.
MongoDB requires `MONGODB_URI`; `MONGODB_DATABASE` defaults to `auraedu_billing`.
`MONGODB_MAX_POOL_SIZE` defaults to 2 per process. Invalid drivers/pool sizes fail
startup. Mongo indexes are installed before readiness; readiness pings the
selected database with a bounded timeout.

Subscription and invoice lifecycle events live inside their aggregates. A partial unique index enforces one trialing subscription per tenant. Plan/subscription references use replica-set transactions and parent version writes, so concurrent deletion cannot orphan a subscription or invoice.

Use a replica set (including Atlas) for the full Mongo adapter. Tests also run
isolated single-document guarantees against standalone Mongo. Pending events
are durable and delivered at least once, with stable event IDs and leases.
Aggregate deletion retains delivery metadata until pending events are dispatched;
MongoDB's document-size limit fails an oversized mutation atomically.

Verification: `GOWORK=off GOFLAGS=-mod=readonly go test -p 1 ./internal/... ./cmd/... ./tests/unit/...`
and `GOWORK=off GOFLAGS=-mod=readonly go vet ./internal/... ./cmd/... ./tests/unit/...`.
