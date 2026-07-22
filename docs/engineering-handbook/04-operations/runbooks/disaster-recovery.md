# Disaster-recovery runbook

Use this runbook only after an incident is declared. It is intentionally provider-aware but avoids copy-paste production credentials and irreversible in-place commands.

## 1. Declare and contain

1. Assign an Incident Commander, Recovery Lead, service Data Owner, Security/Privacy approver and Communications Lead.
2. Record the declaration time, affected services and latest known-good business transaction.
3. Freeze deploys. Disable or drain writes at the gateway/service boundary; do not stop evidence collection.
4. Preserve logs, database identifiers, deployment versions and NATS state. Never delete the affected resources.
5. Select a recovery timestamp before the suspected corruption or loss. Record the expected RPO window.

## 2. Restore PostgreSQL with provider PITR

1. In Render, create a point-in-time recovery as a **new isolated database** at the approved timestamp. Do not overwrite the source.
2. Restrict connectivity to the Recovery Lead and validation workload. Application services remain pointed at the original database.
3. Record the source database, recovery timestamp, new database identifier and provider operation timestamps.
4. Connect using a short-lived credential and validate:
   - PostgreSQL major version and required extensions;
   - Goose migration ledger and expected latest version;
   - row-level-security enablement and FORCE RLS inventory;
   - tenant counts and absence of cross-tenant results;
   - service-specific totals and Data Owner sentinel records;
   - outbox pending/processed counts around the recovery point.
5. Start one isolated application instance against the restored database. Require `/ready`, authentication/authorization and a bounded domain smoke to pass.
6. Security/Privacy, the Data Owner and Incident Commander sign the evidence before cutover.

## 3. Restore from a logical export

Use this path when PITR is unavailable, unsuitable or being independently verified.

1. Select the newest validated encrypted export within the target RPO. Verify object identity, checksum, creation time and retention policy.
2. Inspect it before restore:

   ```bash
   pg_restore --list auraedu.dump
   ```

3. Create a new empty PostgreSQL 18 database with isolated networking.
4. Restore without source ownership or privileges and stop on the first error:

   ```bash
   pg_restore --dbname "$ISOLATED_DATABASE_URL" --no-owner --no-privileges --exit-on-error auraedu.dump
   ```

5. Perform every validation in the PITR section. A valid archive catalogue alone is insufficient.

For safe workstation/CI rehearsal with generated fixtures only, run `bash tools/dr/run-postgres-restore-drill.sh`.

### Verify scheduled PostgreSQL export evidence

1. Confirm the `postgres-backup` Render cron started at minute 17 and its dedicated heartbeat completed within the 75-minute missed-run deadline.
2. Require exactly 25 result objects. Match each database name, object key, SHA-256 digest, byte count and completion timestamp to the heartbeat payload; a partial set is a failed run.
3. In the recovery account, verify every object has AES-256 server-side encryption, checksum metadata, PostgreSQL major `18` metadata and `COMPLIANCE` Object Lock through the recorded retain-until date.
4. If the heartbeat is late or missing, use the dedicated PostgreSQL alert path. Do not accept a NATS heartbeat, Render process exit or partially written object set as evidence of database backup success.
5. During the scheduled quarterly drill, select one retained export, independently recompute its SHA-256, inspect it with `pg_restore --list`, restore into a new isolated PostgreSQL 18 database and complete the migration/RLS/tenant/business validations above.

## 4. Recover NATS JetStream

1. Keep publishers and consumers stopped while assessing broker state.
2. Prefer rebuilding delivery from authoritative database outboxes when the business state is intact and the replay window is bounded.
3. If broker recovery is required, select the independently stored account/stream backup and verify its checksum and timestamp.
4. Restore into an isolated NATS instance. NATS requires conflicting same-name streams to be absent; do not delete production streams to satisfy this condition.
5. Inspect stream subjects, replicas, message counts, durable consumers and delivery positions before application connection.
6. Start consumers before publishers, monitor redeliveries/DLQ volume and prove stable event IDs prevent duplicated business effects.

The persistent Render `/data` disk supports broker operation but is not independent recovery media.

For the isolated generated-fixture rehearsal, run `bash tools/dr/run-nats-restore-drill.sh`. It removes the source broker before restore so the result cannot accidentally come from the original JetStream state.

### Verify scheduled backup evidence

1. Confirm the `nats-backup` Render cron last completed after 02:15 UTC and within the 24-hour delivery-state RPO.
2. Match its emitted object key, SHA-256 digest, byte count and completion timestamp to the external heartbeat record.
3. In the recovery account, verify the object is encrypted and has `COMPLIANCE` Object Lock through the recorded retain-until date. Do not use application credentials for this check.
4. If the heartbeat is late or absent, treat the backup as missed even when Render reports a successful process exit. Page the Recovery Lead and run a supervised replacement backup after diagnosing storage and monitor delivery.
5. Restore only into an isolated broker. Use `nats account restore` against the extracted account directory, then verify stream configuration, message fingerprints and durable consumer state before any worker connects.

## 5. Recreate disposable state

- Provision a clean Valkey instance or flush only the newly provisioned recovery cache.
- Force reauthentication where session continuity is uncertain.
- Let rate limits, page caches and derived views rebuild from authoritative services.
- Regenerate downloadable artifacts only after their source records validate.

## 6. Cut over and roll back

1. Snapshot the exact pre-cutover configuration.
2. Change one affected service/database binding at a time through a reviewed secret/configuration update.
3. Deploy with health-check gating; require `/ready`, authentication, tenant isolation and domain smoke evidence.
4. Re-enable writes gradually. Watch error rate, latency, database connections, outbox lag, consumer redelivery and DLQ growth.
5. If a validation threshold fails, stop writes and revert the binding to the recorded pre-cutover configuration. Do not destroy either database.

## 7. Evidence record

Retain the following in the incident record:

```text
Incident / drill ID:
Declaration time:
Incident Commander / Recovery Lead / Data Owner:
Affected tier and services:
Selected recovery point:
Newest recovered record:
Measured RPO:
Traffic-ready time:
Measured RTO:
Backup/PITR identifiers and checksums:
Migration/RLS/tenant/business validations:
Application and replay smoke results:
Approvals and cutover/rollback decision:
Follow-up owners and due dates:
```

A quarterly drill is complete only when this evidence is retained and failures become owned work items.
