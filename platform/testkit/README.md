# testkit

Shared tenant-isolation and feature-flag fixtures (AURA-2.9/AURA-50).

- `CanonicalTenants` supplies the standard UPSHS/Aboom two-school matrix.
- `NewPostgres` runs service migrations in a disposable PostgreSQL instance.
- `ExecAs` and `QueryAs` bind database work to `app.tenant_id`.
- `AssertNoLeak` checks that records written for one school cannot be observed
  by another.

Repository CI also runs `tools/ci/check-tenant-rls.sh`, which requires every
migration-declared `tenant_id` table to enable and force RLS with an
`app.tenant_id` policy. Service integration tests remain responsible for
non-superuser behavioral proof because structural coverage alone cannot prove
runtime session context.
