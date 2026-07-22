# Chapter 25: API Standards

## Purpose

OpenAPI is the interface law. APIs use consistent errors, pagination, identifiers, tenant context and idempotency semantics.

---

## Scope

Product, engineering, operational and governance decisions directly related to api standards.

---

## Principles

- Prefer explicit, reviewable rules over hidden convention.
- Keep tenant boundaries and human accountability visible.
- Use executable contracts and tests as evidence.

---

## Business Rules

- Changes to api standards must name the stakeholder outcome and operational owner.
- Planned capabilities must not be represented as shipped.
- Exceptions require an ADR or documented product decision.

---

## Technical Rules

- Follow contracts-first delivery and lane ownership.
- Use secure defaults, structured redacted logs and observable failure paths.
- Keep configuration environment-driven and source controlled where appropriate.
- Every OpenAPI document declares a non-empty server list and engineering contact.
- Every operation has a stable `operationId`, meaningful description and globally declared tag.
- Unused parameters and schemas are removed rather than retained as speculative surface area.
- `make contracts-lint` fails on Spectral warnings as well as errors, then proves event and runtime-route parity.

---

## Architecture

The 32 files in `contracts/openapi` are the source of truth for generated Go and TypeScript
clients. `tools/codegen/src/normalize-openapi-metadata.ts` provides a deterministic metadata
check, Spectral enforces the OpenAPI quality profile with warning severity as the failure
threshold, and the route validator compares each public contract operation with its real Go
ServeMux route. Contract generation runs only after those checks pass.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.
- Describe the user-visible or service-visible workflow, not merely the HTTP verb.
- Use domain tags that make generated API documentation navigable.

---

## Examples

- A story affecting api standards updates this chapter and its executable evidence in the same review.
- A design choice records its trade-offs in an ADR before becoming a platform convention.
- `createStaffAssignment` is tagged `assignments`, describes the teacher-scope workflow and
  returns the generated `StaffAssignment` schema.

---

## Anti-patterns

- Duplicating a contract as prose and allowing the copies to drift.
- Claiming completion without tests, monitoring or an owner.
- Creating tenant-specific code paths.
- Suppressing a warning instead of completing or removing the incomplete contract surface.
- Shipping a runtime route that is absent from OpenAPI, or an OpenAPI route that has no handler.

---

## Checklist

- Is the outcome and owner clear?
- Are tenant, permission and feature boundaries covered?
- Are contracts, tests and runbooks updated?
- Is current state distinguished from future state?
- Does `make contracts-lint` report zero warnings and zero errors?
- Do regenerated Go and TypeScript clients compile without drift?

---

## Definition of Done

- The chapter links to authoritative implementation evidence.
- Relevant automated and manual checks pass.
- Any exception or future direction is explicitly recorded.
- Spectral, event-schema validation and public runtime-route parity all pass in the same gate.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
