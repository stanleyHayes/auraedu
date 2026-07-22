# Deployed two-school isolation test

This harness proves tenant isolation at the deployed API boundary. It complements repository and PostgreSQL RLS tests by exercising real gateway routing, authentication and service authorization with two staging schools.

The versioned scenario defines ten representative resource domains. Runtime credentials and real resource IDs are supplied only through `AURA_ISOLATION_SCHOOLS_JSON`; they must never be committed. For every probe, the harness executes both school directions and requires:

- the actor's own resource to return `200`;
- the other school's resource to return the non-enumerating `404` denial;
- a bearer-token and tenant-header mismatch to return `403`.

Run it from `tools/isolationtest` after seeding one owned resource per domain for each staging school:

```sh
export AURA_ISOLATION_BASE_URL=https://staging-api.auraedu.com
export AURA_ISOLATION_RUN_ID=release-2026-07-20-isolation
export AURA_ISOLATION_GIT_SHA=abcdef1234567890
export AURA_ISOLATION_SCHOOLS_JSON='[{"code":"school-a","token":"...","resources":{"student":"..."}},{"code":"school-b","token":"...","resources":{"student":"..."}}]'
GOWORK=off go run . \
  -config ../../release/scenarios/staging-two-school-isolation.json \
  -out ../../release/evidence/records/AURA-50.2/isolation-2026-07-20.json
```

The runtime JSON must include every resource key in the scenario, not only the abbreviated `student` example above. The output file is created with mode `0600` and exclusive-create semantics. It stores only school-code fingerprints and sanitized check metadata—never tokens, tenant codes, resource IDs or response bodies. A failed matrix still writes evidence and exits non-zero so the failed proof can be reviewed without being mistaken for release approval.
