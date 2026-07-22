# Chapter 32: Disaster Recovery

## Purpose

Restore AuraEDU's authoritative education records after infrastructure loss, operator error or regional disruption within explicit recovery objectives. A backup is not trusted until an isolated restore proves its structure and critical records.

---

## Scope

This policy covers the 25 service-owned Render PostgreSQL databases, NATS JetStream delivery state, Valkey cache state, generated files and the configuration required to rebuild the production deployment. It covers recovery governance and evidence; service continuity and routine alert response remain in Chapters 26 and 31.

---

## Principles

- Protect authoritative records before derived state.
- Restore into an isolated target and validate it before traffic moves.
- Never let a broker or cache become the only record of a completed business mutation.
- Keep recovery access separate, time-bound, audited and human approved.
- Rehearse recovery quarterly and after material storage or topology changes.
- State unverified provider capability as a gap, never as shipped protection.

---

## Business Rules

| Data tier | Examples | Maximum RPO | Maximum RTO |
|---|---|---:|---:|
| Tier 0 — safety and money | Identity, Tenant, Admissions, Billing, Fees, Payment, Student | 15 minutes | 2 hours |
| Tier 1 — school operations | Staff, Academic, Attendance, Assessment, Reports, Files, Website, Notifications, CBT | 1 hour | 4 hours |
| Tier 2 — growth and intelligence | CRM, Campaign, Analytics, Knowledge, Assistant, Market Intelligence, AI | 4 hours | 8 hours |
| Delivery state | NATS JetStream streams and durable consumers | 24 hours | 4 hours |
| Rebuildable state | Valkey cache, local build output | No retained-state objective | 1 hour |

RPO is measured from the newest validated recovered record to the declared incident time. RTO is measured from incident declaration to validated service restoration. The Incident Commander may accept a degraded read-only mode, but it does not stop the RTO clock unless the affected business owner signs off.

The Incident Commander owns safety, priority and communication. The Recovery Lead owns commands and evidence. Each service Data Owner validates record semantics and tenant boundaries. Security and Privacy approve access to restored personal data. The Communications Lead coordinates internal, school and regulatory updates.

---

## Technical Rules

- Production PostgreSQL must use paid managed plans with point-in-time recovery capability; the blueprint pins PostgreSQL 18 for every service database.
- Tier 0 requires provider PITR plus a logically independent custom-format export. Tier 1 and Tier 2 require provider recovery plus scheduled logical exports according to their RPO.
- Logical exports use `pg_dump --format=custom --no-owner --no-privileges`; validation requires `pg_restore --list` and an isolated restore with `--exit-on-error`.
- The `postgres-backup` cron starts hourly at minute 17, exports all 25 service databases through private connection strings and must finish within 55 minutes. Validated connection URLs are translated into libpq environment variables; database credentials never enter process arguments or logs.
- Backup objects must be encrypted, immutable during retention, stored outside the application runtime and inaccessible to application credentials.
- NATS runs on a persistent `/data` disk. Account/stream backup artifacts must be copied to independent encrypted object storage; the JetStream disk alone is not a backup.
- The `nats-backup` cron runs daily at 02:15 UTC and must complete an account-level backup with consumers, integrity checks, SHA-256 evidence, AES-256 server-side encryption and compliance retention in the dedicated recovery bucket.
- Recovery storage uses a write-only principal that is unavailable to application services. The backup process cannot delete, shorten retention or overwrite an existing object.
- A successful run is not complete until the external heartbeat monitor accepts its signed status. The PostgreSQL monitor pages after a 75-minute missed-run deadline and the NATS monitor after 26 hours; execution failures use separate alert endpoints so one backup cannot mask the other.
- Transactional outboxes remain the authoritative bridge from database state to events. Recovered consumers must tolerate replay and stable event identifiers.
- Valkey is disposable. Sessions, rate limits and caches are recreated; no business record may depend on restoring Valkey.
- Recovery must not overwrite the affected database in place. Restore to a new isolated database, validate it, then switch service configuration through a reviewed change.

---

## Architecture

```text
service transaction -> service PostgreSQL -> provider PITR
                                    |-----> hourly postgres-backup cron
                                                  |-> validated custom-format exports
                                                  |-> compliance-locked recovery bucket
                                                  |-> dedicated heartbeat / failure alert
                                    |-----> transactional outbox -> NATS JetStream
                                                                      |-> daily account backup cron
                                                                                 |-> compliance-locked recovery bucket
                                                                                 |-> success heartbeat / failure alert

restore target -> structural checks -> tenant/data-owner checks -> service smoke -> traffic switch
```

The local executable drill at `tools/dr/run-postgres-restore-drill.sh` creates a source database, takes a custom-format backup, validates its catalogue, restores into a separate target and compares a deterministic content fingerprint. `tools/dr/run-nats-restore-drill.sh` destroys its source broker before restoring a file-backed stream, messages and durable consumer into a clean JetStream target. The scheduled implementations live at `tools/dr/postgres-backup` and `tools/dr/nats-backup`: the first exports and catalogue-validates every service database before immutable upload; the second invokes the official pinned NATS CLI account backup and verifies a non-empty stream payload. Both hash their artifacts, sign S3-compatible uploads with SigV4, require Object Lock compliance retention and fail if their dedicated monitor does not accept success. CI checks this fail-closed behavior and the Render topology, but it never connects to production.

---

## Best Practices

- Run the local restore drill on every recovery-tool change and retain its JSON result with CI evidence.
- Run a provider restore rehearsal quarterly, rotating across Tier 0 databases so all are exercised annually.
- Record incident time, selected recovery point, backup identity, operators, approvals, elapsed time, checks and final disposition.
- Validate migration ledger, row-level security, tenant counts, financial totals and at least one Data Owner sentinel before cutover.
- Restore NATS only after authoritative databases and outbox workers are stable; replay from the database when safer than restoring broker state.
- Configure the PostgreSQL heartbeat monitor with a 75-minute deadline and the NATS monitor with a 26-hour deadline and two-hour grace window. Route missed and explicit failure alerts to the production on-call receiver, not a personal channel.
- Enable Object Lock on the recovery bucket before its name or credentials are supplied. A bucket that rejects compliance headers must make the cron fail.

---

## Examples

Run the safe local PostgreSQL rehearsal:

```bash
bash tools/dr/run-postgres-restore-drill.sh
bash tools/dr/run-postgres-backup-smoke.sh
bash tools/dr/run-nats-restore-drill.sh
GOWORK=off go test ./... # from tools/dr/nats-backup
GOWORK=off go test ./... # from tools/dr/postgres-backup
```

The successful final line is machine-readable and includes the restored fingerprint, backup size and elapsed seconds. It is development evidence, not proof that Render PITR or off-site retention is configured.

---

## Anti-patterns

- Calling a persistent disk, read replica or untested snapshot a backup.
- Restoring directly over production or changing all 25 database URLs at once.
- Treating a successful command exit as data validation.
- Keeping exports beside the database or under the same runtime credential.
- Letting a backup job report success before off-site retention and the heartbeat acknowledgement complete.
- Using application Cloudinary, database or NATS credentials for the recovery bucket.
- Restoring cached data before authoritative records.
- Claiming an RPO or RTO from documentation without a timestamped drill.

---

## Checklist

- [ ] Incident Commander and Recovery Lead are named.
- [ ] Writes and deploys are frozen for affected services.
- [ ] Recovery point and data-loss window are documented.
- [ ] Restore target is isolated from production traffic.
- [ ] Schema, migration ledger, RLS, tenant and business sentinels pass.
- [ ] Service `/ready`, contract smoke and outbox replay checks pass.
- [ ] Data Owner, Security/Privacy and Incident Commander approve cutover.
- [ ] Old environment remains quarantined for rollback and investigation.
- [ ] Timeline, evidence, decisions and follow-up work are retained.

---

## Definition of Done

- Local custom-format backup and isolated restore drill passes in a pinned PostgreSQL 18 container.
- Local JetStream backup survives source destruction and restores its stream, messages and durable consumer into a pinned clean broker.
- CI proves every declared database uses the pinned paid-plan topology and NATS retains a minimum 10 GB `/data` disk.
- Render declares isolated hourly `postgres-backup` and daily `nats-backup` crons plus dedicated monitoring secrets; tests prove full database coverage, structural validation, immutable encrypted upload, integrity metadata and fail-closed monitor configuration.
- A current runbook assigns roles, commands, validation and rollback.
- A timestamped production-provider drill demonstrates the targets for a Tier 0 database and a NATS recovery path.
- Monitoring alerts when scheduled exports or recovery drills miss policy.

**Provider evidence still required:** Render PITR configuration, successful deployed PostgreSQL and NATS cron runs, recovery-bucket versioning/Object Lock and retained-object inspection, received dedicated heartbeat and alert delivery, and a provider-hosted restore/cutover rehearsal must be verified before AURA-9.4 is Done.

---

## References

- [Disaster-recovery runbook](runbooks/disaster-recovery.md)
- [Monitoring and alert response](31-monitoring.md)
- [Render backup and restore guidance](https://render.com/articles/how-to-backup-and-restore-postgresql-databases)
- [NATS JetStream disaster recovery](https://docs.nats.io/running-a-nats-service/nats_admin/jetstream_admin/disaster_recovery)
- [Agent execution plan](../../../agent_plan.md)
