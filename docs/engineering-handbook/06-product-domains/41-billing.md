# Chapter 41: Billing

## Purpose

Billing controls subscriptions and plan eligibility while payment services handle provider transactions and reconciliation.

---

## Scope

Product, engineering, operational and governance decisions directly related to billing.

---

## Principles

- Prefer explicit, reviewable rules over hidden convention.
- Keep tenant boundaries and human accountability visible.
- Use executable contracts and tests as evidence.

---

## Business Rules

- Changes to billing must name the stakeholder outcome and operational owner.
- Planned capabilities must not be represented as shipped.
- Exceptions require an ADR or documented product decision.

---

## Technical Rules

- Follow contracts-first delivery and lane ownership.
- Use secure defaults, structured redacted logs and observable failure paths.
- Keep configuration environment-driven and source controlled where appropriate.

---

## Architecture

This chapter links the relevant platform boundaries to contracts, diagrams, implementation directories and operational controls for billing.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- A story affecting billing updates this chapter and its executable evidence in the same review.
- A design choice records its trade-offs in an ADR before becoming a platform convention.

---

## Anti-patterns

- Duplicating a contract as prose and allowing the copies to drift.
- Claiming completion without tests, monitoring or an owner.
- Creating tenant-specific code paths.

---

## Checklist

- Is the outcome and owner clear?
- Are tenant, permission and feature boundaries covered?
- Are contracts, tests and runbooks updated?
- Is current state distinguished from future state?

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
