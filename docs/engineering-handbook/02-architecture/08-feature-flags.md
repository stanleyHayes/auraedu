# Chapter 08: Feature Flags

## Purpose

A disabled feature is hidden in the UI, rejected by APIs and skipped by workers. The same entitlement decision must remain safe when Tenant Service is slow, unavailable or returns malformed data.

---

## Scope

Product, engineering, operational and governance decisions directly related to feature flags.

---

## Principles

- Tenant Service is the production authority for live entitlements.
- Unknown tenants, unknown feature keys and unavailable production authority fail closed.
- Feature checks are tenant-specific authorization inputs, not UI preferences.
- Dependency calls are bounded in time and response size.
- Use executable contracts and tests as evidence.

---

## Business Rules

- A plan, trial or operator override may enable only keys present in `contracts/features/features.yaml`.
- Disabling a feature must remove navigation, reject direct API access and prevent worker execution.
- Development may use reviewed registry fixtures so the local stack remains usable. Production may never grant an entitlement from those fixtures when the authority is missing or unhealthy.
- Planned capabilities must not be represented as shipped.

---

## Technical Rules

- `contracts/features/features.yaml` is the key registry; Tenant Service stores each tenant's live snapshot.
- The shared Go runtime gate calls Tenant Service through `platform/flags`; Python services implement the same production fail-closed semantics.
- The Go entitlement lookup has a four-second total client timeout, safely encodes the tenant as one query value and reads at most 1 MiB before decoding.
- A missing binding, network error, non-`200` status, oversized body or malformed response selects the configured fallback. The production fallback always denies; development may use the checked-in registry snapshot.
- Actor headers are derived from authenticated context. Callers may not synthesize an entitlement by trusting request-body, query or user-supplied identity headers.
- The requested tenant and returned snapshot must remain inside the same authoritative tenant boundary.

---

## Architecture

```text
UI navigation ─┐
API handler ───┼─ feature key + authenticated tenant ──> runtime gate
worker job ────┘                                          │
                                                         ├─ production: live Tenant Service or deny
                                                         └─ development: live service or registry fixture
```

The gateway performs an early route-level feature check, but every owning service repeats the check before domain work. Workers check before consuming or mutating feature-owned state. This defence in depth prevents a hidden navigation item from becoming the only control.

---

## Best Practices

- Pass the request context so caller cancellation is observed in addition to the client timeout.
- Keep feature responses small, deterministic and free of personal data.
- Test enabled, disabled, unknown, malformed, timeout and production-outage paths.
- Alert on sustained entitlement dependency errors; do not extend timeouts until stalled requests consume the service.

---

## Examples

- Tenant `upshs` has `online_payments=true`: navigation appears, Gateway admits the route and Payment Service performs its own live check.
- Tenant Service stalls: the lookup returns after the bounded client deadline. Production denies the feature; development may use its reviewed fixture.
- A tenant value contains reserved URL characters: it is encoded as one `tenant` query value and cannot add or replace parameters.

---

## Anti-patterns

- Using `http.DefaultClient` or a zero-timeout client for entitlement resolution.
- Falling back to a checked-in enabled value in production.
- Hiding a navigation link while leaving the API or worker unguarded.
- Concatenating a tenant code into a URL without query encoding.
- Accepting an unbounded or partially decoded response from the authority.
- Creating tenant-specific code paths.

---

## Checklist

- Is the key registered and plan ownership explicit?
- Do UI, API and worker boundaries all enforce it?
- Does production fail closed for missing, slow, oversized and malformed authority responses?
- Is the dependency call bounded and tenant input safely encoded?
- Do tests prove enabled, disabled and outage behavior?

---

## Definition of Done

- The feature key and generated registries are synchronized.
- Tenant Service persists and returns the live tenant snapshot.
- Gateway, owning services, workers and frontends apply the same key.
- Shared runtime timeout, response bound, URL encoding and fail-closed regressions pass.
- Relevant telemetry and operator ownership exist.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
- [Go runtime gate](../../../platform/flags)
