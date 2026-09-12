# AuraEDU single-container demo

AURA-9.12 runs the 18 Go domain/identity services in `services.txt`, their 18
worker commands, the gateway, Redis and NATS in one container. PostgreSQL remains
the default; MongoDB is an explicit configuration choice. This is a development
demo topology, not the production deployment or a guarantee of hosted free-tier
capacity. Historical server-only memory figures do not measure the worker fleet.

## Build and configure

```sh
docker build -f deploy/demo/Dockerfile -t auraedu-demo:mongo-local .
```

Provide fresh `JWT_SIGNING_KEY` and `INTERNAL_SERVICE_TOKEN` secrets explicitly.
The entrypoint refuses to substitute predictable demo credentials.

| Driver | Required environment | Database isolation |
|---|---|---|
| PostgreSQL (default) | `DATABASE_URL` | Service-specific `DATABASE_SCHEMA`, unchanged |
| MongoDB | `DATABASE_DRIVER=mongodb`, `MONGODB_URI` | `${MONGODB_DATABASE_PREFIX}_${service}`, prefix defaults to `auraedu_demo` |

For example, student and identity own `auraedu_demo_student` and
`auraedu_demo_identity`. Every Mongo server/worker pool defaults to two connections.
The **full demo requires a replica set or mongos**: billing, admissions and other
multi-document operations require transactions and refuse standalone MongoDB.
Atlas Free uses a replica set; standalone MongoDB test fixtures are a separate
single-document-atomicity test configuration, not an Atlas emulator.

```sh
docker run --name auraedu-demo -p 127.0.0.1:8080:8080 \
  -e DATABASE_DRIVER=mongodb -e MONGODB_URI \
  -e JWT_SIGNING_KEY -e INTERNAL_SERVICE_TOKEN \
  -e NOTIFICATION_PROVIDER=mock auraedu-demo:mongo-local
```

Set the exported variables before running the command. For PostgreSQL replace the
Mongo variables with `-e DATABASE_URL`. Do not enable external notification
providers for synthetic fixtures. Only the gateway binds externally; internal
HTTP, Redis and NATS listeners bind loopback. Services run as an unprivileged user.
The supervisor waits for all HTTP readiness endpoints, launches workers on distinct
ports, monitors actual child PIDs, and stops the entire instance when any exits.
Shutdown grants 15 seconds before terminating remaining processes.

## Repeatable local Mongo verification and fixture bootstrap

```sh
python3 tools/smoke/demo-mongodb.py
```

This creates an isolated Docker internal network and Mongo replica set, seeds two
synthetic tenant/feature fixtures, starts the built demo, creates explicit random
school-admin credentials with identity's `seed-demo` command, and checks real
login, student creation, tenant isolation, a disabled feature, durable outbox
processing and audit consumption, refresh rotation/replay denial, logout revocation, and record
persistence across a demo restart. It also submits and approves a real onboarding request, captures the invitation
through the real Notification service and a local SMTP receiver, accepts it,
logs in as the activated administrator, and checks billing worker provisioning.
The internal Docker network and SMTP capture prevent external sends. Gateway
requests run through a local HTTP test client using `docker exec`; the smoke
publishes no host ports. The demo is capped at 512 MiB and reports measured
memory, worker count and listener isolation. A final
worker termination verifies that the supervisor fails the entire container. The script cleans only the resources whose random names it created.
`KEEP_DEMO_SMOKE=1` retains those resources for diagnosis; `DEMO_IMAGE` overrides
the image. These are smoke fixtures, not a complete seeded school. The smoke creates one synthetic billing plan
before onboarding; real onboarding requires a configured billing plan catalog.

To create a login for an existing development tenant manually:

```sh
docker exec \
  -e MONGODB_DATABASE=auraedu_demo_identity \
  -e DEMO_TENANT_ID -e DEMO_USER_EMAIL -e DEMO_USER_PASSWORD \
  -e DEMO_USER_ROLE=school_admin auraedu-demo identity-service seed-demo
```

`docker exec -e NAME` forwards that variable from your shell. Use a unique email
and a password of at least 12 characters. `seed-demo` is development-only and
never replaces an existing account's credentials. It creates identity records;
it does not create the tenant, billing catalog or academic setup. Login through
`POST /api/v1/auth/login` with `X-Tenant-ID` and JSON `email`/`password`.

The existing PostgreSQL bootstrap remains `go run ./tools/seed`, configured with
its service-specific database URLs; it does not bootstrap MongoDB.

## Local verification

On 2026-09-12 the complete smoke passed with all 18 servers, all 18 workers and
the gateway. The demo container reported **174.9 MiB under its 512 MiB cap** at
startup, with listener isolation and every workflow assertion passing. This is a
synthetic local workload measurement, not a hosted capacity guarantee.

## Limits and verification scope

Mongo/PostgreSQL business records live in the configured external database.
Redis, NATS and local uploaded/generated file bytes are ephemeral in this image;
record persistence does **not** prove file persistence. Configure the appropriate
external media/storage adapter for durable files. NATS is not a permanent audit
log, and an outbox acknowledged before NATS state disappears cannot recreate
that lost broker history. Consumers must maintain their own durable state.

The growth/marketing services listed in `deploy/undeployed-services.txt` and the
Python AI services are omitted. Their features must remain disabled for demo
tenants. No hosted Atlas, Render, provider-delivery, backup/recovery or complete
school-workflow verification is implied by a local Mongo smoke.
