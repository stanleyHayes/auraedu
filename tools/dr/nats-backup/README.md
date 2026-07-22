# JetStream off-site backup job

`nats-backup` is the fail-closed implementation behind the daily Render cron. It runs the pinned official NATS CLI account backup, includes durable consumers, rejects an empty result, creates a gzip archive and SHA-256 evidence, and uploads the object through an AWS SigV4-signed `PutObject` request.

## Recovery account prerequisites

Create the recovery bucket in an account or project whose credentials are not available to AuraEDU application services.

1. Enable bucket versioning and Object Lock when the bucket is created.
2. Require server-side encryption and retain objects in `COMPLIANCE` mode for at least 35 days.
3. Give the cron principal only the write permissions needed for the configured prefix: `s3:PutObject` and `s3:PutObjectRetention`. Do not grant object deletion, retention bypass or application-runtime access.
4. Configure lifecycle expiry after the approved retention period. Lifecycle must not shorten Object Lock.
5. Configure the heartbeat receiver to page when no accepted success arrives within 26 hours, with a maximum two-hour grace period.
6. Route the separate failure endpoint to the production on-call receiver and test both firing and resolution evidence.

The upload is conditional (`If-None-Match: *`), uses signed payload integrity, requests `AES256` encryption and requests an explicit Object Lock retain-until timestamp. A bucket that does not support or authorize those controls rejects the job.

## Required Render configuration

The `auraedu-dr-secrets` group owns these values:

- `DR_BACKUP_S3_ENDPOINT`
- `DR_BACKUP_S3_REGION`
- `DR_BACKUP_S3_BUCKET`
- `DR_BACKUP_S3_PREFIX`
- `DR_BACKUP_S3_ACCESS_KEY_ID`
- `DR_BACKUP_S3_SECRET_ACCESS_KEY`
- `DR_BACKUP_RETENTION_DAYS`
- `DR_BACKUP_HEARTBEAT_URL` and `DR_BACKUP_HEARTBEAT_TOKEN`
- `DR_BACKUP_ALERT_URL` and `DR_BACKUP_ALERT_TOKEN`

`NATS_URL` comes only from Render's private `nats` service. The process refuses plaintext object-store or monitor URLs, credentials embedded in URLs, retention below 30 days, missing monitor tokens and excessive deadlines.

## Evidence and restore

Every success heartbeat contains the object key, SHA-256 digest, byte count and completion timestamp. Match those values to the recovery bucket before accepting a run as evidence.

Extract the selected archive into an isolated environment and use `nats account restore <account-directory>`. Follow the validation and cutover rules in the [disaster-recovery runbook](../../../docs/engineering-handbook/04-operations/runbooks/disaster-recovery.md); never restore over the active broker.

Run the local checks with:

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
docker build -f infrastructure/docker/nats-backup.Dockerfile -t auraedu/nats-backup:verify .
```
