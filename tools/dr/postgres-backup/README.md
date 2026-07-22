# AuraEDU PostgreSQL logical backup

This command creates ownership-neutral PostgreSQL 18 custom-format exports for every service database, validates each export catalogue with `pg_restore --list`, hashes it and uploads it to independent S3-compatible storage with server-side encryption, non-overwrite semantics and `COMPLIANCE` Object Lock.

The Render cron supplies `POSTGRES_DATABASES` as a comma-separated allowlist. Each name resolves to `POSTGRES_<NAME>_DATABASE_URL`; each URL is validated and translated into libpq's dedicated `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` and approved TLS environment variables, never command arguments. A run succeeds only after every configured database is exported, structurally validated and stored, and the dedicated authenticated success heartbeat accepts the result. Any partial failure sends the dedicated alert and leaves the already-written immutable objects available for investigation.

The recovery bucket must have versioning and Object Lock enabled before deployment. Its principal should be isolated from application credentials and limited to `s3:PutObject` and `s3:PutObjectRetention`; it must not have delete, lifecycle-policy or retention-bypass permission. Configure the PostgreSQL missed-run monitor for a 75-minute deadline because the cron starts hourly at minute 17 and has a 55-minute fail-closed job timeout.

Restore an export only into a new isolated PostgreSQL 18 target. Verify the recorded SHA-256, run `pg_restore --list`, then restore with `pg_restore --no-owner --no-privileges --exit-on-error`. The full recovery and cutover procedure is in `docs/engineering-handbook/04-operations/runbooks/disaster-recovery.md`.

Required environment:

- `POSTGRES_DATABASES` and each corresponding `POSTGRES_<NAME>_DATABASE_URL`
- `DR_BACKUP_S3_ENDPOINT`, `DR_BACKUP_S3_REGION`, `DR_BACKUP_S3_BUCKET`, `DR_BACKUP_S3_PREFIX`
- `DR_BACKUP_S3_ACCESS_KEY_ID`, `DR_BACKUP_S3_SECRET_ACCESS_KEY`, optional `DR_BACKUP_S3_SESSION_TOKEN`
- `DR_BACKUP_RETENTION_DAYS` (minimum 30; default 35)
- `DR_POSTGRES_BACKUP_HEARTBEAT_URL`, `DR_POSTGRES_BACKUP_HEARTBEAT_TOKEN`
- `DR_POSTGRES_BACKUP_ALERT_URL`, `DR_POSTGRES_BACKUP_ALERT_TOKEN`
- optional `DR_POSTGRES_BACKUP_JOB_TIMEOUT` (default 55 minutes, maximum one hour) and `DR_BACKUP_HTTP_TIMEOUT`
