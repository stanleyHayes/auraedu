# Chapter 26: DevOps

## Purpose

DevOps defines the shared rules, boundaries and evidence for this part of the AuraEDU education operating system.

---

## Scope

Product, engineering, operational and governance decisions directly related to devops.

---

## Principles

- Prefer explicit, reviewable rules over hidden convention.
- Keep tenant boundaries and human accountability visible.
- Use executable contracts and tests as evidence.

---

## Business Rules

- Changes to devops must name the stakeholder outcome and operational owner.
- Planned capabilities must not be represented as shipped.
- Exceptions require an ADR or documented product decision.

---

## Technical Rules

- Follow contracts-first delivery and lane ownership.
- Use secure defaults, structured redacted logs and observable failure paths.
- Keep configuration environment-driven and source controlled where appropriate.
- Database migration history is additive and service-local. `make migrate-check` must cover every
  history and executable runner; a maintenance run uses a mode-0600, untracked URL map and stops
  on the first service failure without printing credentials.

---

## Architecture

Every service owns its schema and migration ledger. The root migration orchestrator discovers all
26 histories, validates 127 ordered SQL files, and invokes the owning Go or Python runner rather
than applying SQL through a shared superuser. Service runners retain their advisory locks and
idempotent ledgers. Normal service boot remains migration-safe; the root command exists for an
explicit, auditable maintenance window and bounded recovery operations.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- A story affecting devops updates this chapter and its executable evidence in the same review.
- A design choice records its trade-offs in an ADR before becoming a platform convention.

---

## Anti-patterns

- Duplicating a contract as prose and allowing the copies to drift.
- Claiming completion without tests, monitoring or an owner.
- Creating tenant-specific code paths.
- Passing database credentials on a command line, committing a URL map, or continuing to another
  service after a migration failure.

---

## Checklist

- Is the outcome and owner clear?
- Are tenant, permission and feature boundaries covered?
- Are contracts, tests and runbooks updated?
- Is current state distinguished from future state?
- Does `make migrate-check` cover every migration directory and runner without connecting to a DB?
- Is the credential file private, untracked and scoped to the selected maintenance operation?

---

## Definition of Done

- The chapter links to authoritative implementation evidence.
- Relevant automated and manual checks pass.
- Any exception or future direction is explicitly recorded.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
