# AURA-9.12 Mongo demo continuation

Owner: Codex coordinator. Started 2026-09-12. Status: complete for the 18-service local demo scope.

## Scope and decision

Complete selectable persistence for the 18 Go services in `deploy/demo/services.txt`, their workers, and the combined demo image. PostgreSQL remains the default. No existing PostgreSQL data is automatically copied by changing `DATABASE_DRIVER`.

The earlier assumption that Atlas Free cannot run transactions was incorrect. [Atlas Free has three replica nodes](https://www.mongodb.com/docs/atlas/reference/free-shared-limitations/); [standalone MongoDB cannot run multi-document transactions](https://www.mongodb.com/docs/manual/core/transactions-production-consideration/). The complete demo requires a replica set. Single-document identity and outbox tests also run against standalone MongoDB deliberately.

Mongo application scoping supplements authorization; it is not PostgreSQL database-enforced RLS. Shared mutation guards prohibit changes to tenant ownership, and aggregation cannot introduce another collection outside its initial tenant scope.

## Ownership

| Lane | Exclusive work | Status |
| --- | --- | --- |
| Identity | Identity adapter, security transitions, server/worker/migrate, explicit development seed | Implemented; real Mongo security suite including delete/recreate, existing PostgreSQL integration, package checks and strict lint passed |
| Domain A, first phase | Report, payment, billing | Implemented; atomicity, generation fencing, parent-reference races, module checks and vet passed |
| Domain B, first phase | CBT, admissions, notification | Implemented; real Mongo concurrency/rollback and standalone-refusal suites, module builds and strict lint passed |
| Domain A, second phase | Academic, analytics, attendance, tenant, website | Implemented; Mongo conformance, concurrency/rollback regressions, strict lint and full eight-service vet passed |
| Domain B, second phase | Student, staff, file, fees, assessment, audit | Implemented; adversarial tests, all nine lane module builds and strict lint passed; file tombstone/outbox regression passed |
| Platform/demo | Shared Mongo guards/transactions/testkit; demo supervision, image, smoke and runbook | Shared regression suite, final image build and full isolated Mongo smoke passed |

## Evidence

- Before changes, all 11 existing PostgreSQL/Mongo adapter conformance suites passed. All packages for those services also passed. Review nevertheless found atomicity and event-delivery omissions, so those tests alone were not accepted as completion evidence.
- Identity: concurrent MFA counter replay, refresh replay revoking the successor family, atomic password reset/session revocation, competing invite acceptance, role/session/outbox commit, injected write rollback, retained refresh ancestors, durable inactive status, tenant/platform isolation, recoverable onboarding lease fencing, and account delete/recreate without reviving old sessions or losing queued events pass on real standalone Mongo.
- Report/payment/billing: concurrent reconciliation, stable ledger and events, terminal payment protection, trial uniqueness, generation lease fencing, and parent-delete/child-create races pass. Report and billing require replica transactions for cross-document references.
- CBT/admissions/notification: tenant isolation, duplicate submission/admission/enrollment races, concurrent claims, failed enrollment rollback, feedback replay, monotone delivery state, and tenant suppression pass against real Mongo replica sets.
- Student/staff/file/fees/assessment/audit: Mongo conformance and targeted parent/uniqueness, transactional rollback, enrollment event delivery, cross-invoice payment deduplication and receipt-reference tests passed. File deletion additionally hides tombstones, preserves pending events, and carries the cleanup path through the outbox. Final notification feedback regression passed after lint refactoring.
- Academic/analytics/attendance/tenant/website: conformance plus concurrent overlap denial, attendance bulk rollback, analytics idempotence and weighted projections, atomic website provisioning/cascade, onboarding event delivery and domain visibility passed. Tenant code reuse restores default features without reviving old domains. Final post-refactor academic / attendance / tenant suites passed in 3.309 s / 2.470 s / 2.869 s.
- Both domain lanes cross-reviewed the other lane; findings were fixed and regression-tested before source freeze. Identity post-refactor Mongo, worker, server and application suites passed.
- Full-image verification exposed mutable Mongo index option reuse on repeated server/worker startup. Student and tenant now allocate independent builders; 34 sequential/concurrent student initializations and 16 repeated tenant initializations passed. All other demo index definitions were checked.
- Full gateway login exposed an existing JWT grant spelling mismatch. The separately reviewed `platform/auth` compatibility fix reads canonical `permissions` and legacy `perms`, rejects conflicting aliases, and signs the canonical spelling. Auth and gateway tests passed; route authorization remains enforced.
- Shared verification: `TESTCONTAINERS_RYUK_DISABLED=true GOFLAGS=-mod=readonly go test -p 1` for Mongo/store and the Mongo testkit regressions passed (64.211 s / 0.736 s / 13.744 s). This includes scope confinement, update/aggregation bypass denial, embedded outbox retry, nested transaction rollback/commit, and standalone transaction refusal.
- Parallel Docker workloads caused reaper-name collisions and slow daemon responses. Redundant suites were stopped; final database verification is serialized with explicit container cleanup. Infrastructure setup failures are not recorded as passing assertions.

## Final gates

| Gate | State |
| --- | --- |
| All 18 servers, workers and migrate commands select the configured driver | Complete source inventory for all 18 services |
| Prior multi-write adapter defects have rollback/concurrency regressions | Passed targeted transactional rollback, parent-reference races, tenant reseeding and deletion/outbox regressions |
| Independent module builds, relevant tests/vet, formatting | Passed: all 18 service lanes strict new-diff lint, module checks and relevant tests; shared lint; smoke Ruff, mypy strict, Pyright; entrypoint shellcheck |
| Final demo image builds from final source | Passed; image `auraedu-demo:mongo-local`, ID below |
| All service readiness, real login, domain write, tenant isolation and disabled-feature denial | Passed; 18 servers and 18 workers, actual gateway login/write, cross-tenant 404 and feature-disabled 403 |
| Worker publication and downstream persistence | Passed; student outbox drained and audit consumer persisted the event |
| Onboarding approval, invitation acceptance and usable login | Passed; real Notification service to local SMTP capture, accepted invite, activated login and billing worker subscription |
| Refresh replay/logout denial and persistence after restart | Passed; old and successor refresh denied after replay, logout denied refresh, student retained after container restart |
| Supervisor detects a failed child and exits | Passed; killing student worker caused nonzero container exit |

## Reproduction and final artifact

```sh
docker build -f deploy/demo/Dockerfile -t auraedu-demo:mongo-local .
python3 tools/smoke/demo-mongodb.py
```

Final smoke exited 0 on 2026-09-12. The image runs as `auraedu` and measured 174.9 MiB under an explicit 512 MiB memory limit; this is a smoke observation, not a load-test capacity claim. All non-gateway listeners were verified loopback-only. Synthetic resources used an internal Docker network and local SMTP capture, and were removed afterward.

- Image ID: `sha256:0cd76fd34e12a6977a56bce6863d7bd330de27490863b44551fbd534c9973ae3`.
- Build log for this local run: `/tmp/auraedu-demo-build-final.log`.
- Smoke log for this local run: `/tmp/auraedu-demo-mongo-smoke.log`.
- Configuration/bootstrap instructions: [demo runbook](../../deploy/demo/README.md).

Hosted Atlas/Render verification, production deployment, data migration from PostgreSQL, and external delivery-provider verification are outside this local demo completion. Local upload/PDF bytes remain ephemeral unless persistent object storage is configured; durable Mongo metadata does not make those bytes persistent.
