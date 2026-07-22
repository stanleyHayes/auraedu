# Chapter 02: Business Model

## Purpose

Define how AuraEDU creates, delivers and captures sustainable value without compromising education outcomes.

---

## Scope

Plans, subscriptions, enterprise agreements, marketplace options, white labelling, revenue, costs, growth and operating KPIs.

---

## Principles

- Price the value delivered, not the amount of data captured.
- Keep the entry path simple for schools.
- Never hold school records hostage during a downgrade.
- Enterprise flexibility must not create tenant-specific forks.
- Growth must preserve support quality and platform reliability.

---

## Business Rules

- Plans progress from Starter to Growth, Professional, AI Plus and Enterprise.
- Trials and onboarding offers must have explicit duration, conversion and data-retention terms.
- Downgrades preserve history; restricted capabilities become read-only or hidden according to policy.
- Marketplace and partner revenue requires security, contract and support review.
- Published prices and claims must come from an approved commercial source, never placeholder code.

---

## Technical Rules

- Billing owns plan and subscription state; feature availability is enforced by the tenant feature service.
- Payment processing is separated from SaaS billing records.
- Revenue and usage metrics must be tenant-aware and exclude sensitive learner details.

---

## Architecture

Tenant creation emits an event that can establish a trial subscription. Billing controls plan eligibility; tenant feature overrides remain audited.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- Starter supports core school records and daily operations; higher tiers add finance, analytics, advanced channels and AI.
- Enterprise may include integrations and an SLA without receiving a private codebase.

---

## Anti-patterns

- Hardcoding prices in the marketing UI.
- Deleting historical records immediately after downgrade.
- Treating every custom request as an enterprise feature.

---

## Checklist

- Is the offer approved and current?
- Does the entitlement map to feature keys?
- Is downgrade behaviour explicit?
- Are unit economics and support costs measurable?

---

## Definition of Done

- Plan definitions, feature mappings and public copy agree.
- Subscription transitions have tests and audit events.
- Commercial claims have an owner and review date.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
