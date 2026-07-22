# Migration orchestrator

`make migrate-check` inventories every service-local SQL history and fails when numbering is
non-contiguous, names are invalid, a forward marker is missing, a migration is empty, or a service
has no executable Go/Python runner. It does not connect to a database.

`make migrate` applies the same validated inventory sequentially and stops at the first failure.
It requires `AURA_MIGRATION_DATABASE_URLS_FILE` to point to a mode-`0600`, non-versioned JSON file:

```json
{
  "academic-service": "postgresql://user:password@host/academic?sslmode=require",
  "student-service": "postgresql://user:password@host/student?sslmode=require"
}
```

The complete run requires one entry for each service printed by `make migrate-check`. A bounded
maintenance operation may select one or more services directly:

```sh
AURA_MIGRATION_DATABASE_URLS_FILE=/secure/auraedu-databases.json \
  node tools/migrations/orchestrate.mjs --service student-service
```

The orchestrator never prints a database URL, refuses group/world-readable secret files, rejects
tracked secret files, accepts PostgreSQL URLs only, uses each service's advisory-lock-protected
ledger, imposes a five-minute service timeout, and never continues after a failed migration.
Use `--dry-run` to print the exact service/version plan without reading secrets or connecting.
