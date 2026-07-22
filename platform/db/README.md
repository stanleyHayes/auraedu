# `platform/db`

The shared PostgreSQL adapter for Go services. `Open` creates a pgx pool, serializes first-run Goose migrations with an advisory lock, and returns a `DB` wrapper for transactions and queries.

All request-scoped operations apply the tenant ID or the explicit platform-admin session flag before SQL runs. `SetTenantID`, `SetPlatformAdmin`, and `ResetTenantID` are available for adapters that manage their own pgx transaction.

Never disable row-level security, reuse a tenant-bound transaction for another tenant, or query another service's database. Migrations remain additive and service-local. Run `go test ./db`; integration tests require a working Docker daemon for their disposable PostgreSQL container.
