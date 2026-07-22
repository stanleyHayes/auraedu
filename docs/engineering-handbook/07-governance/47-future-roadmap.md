# Chapter 47: Future Roadmap

## Purpose

The roadmap separates committed delivery, discovery, post-MVP options and explicitly rejected directions.

---

## Scope

Product, engineering, operational and governance decisions directly related to future roadmap.

---

## Principles

- Prefer explicit, reviewable rules over hidden convention.
- Keep tenant boundaries and human accountability visible.
- Use executable contracts and tests as evidence.

---

## Business Rules

- Changes to future roadmap must name the stakeholder outcome and operational owner.
- Planned capabilities must not be represented as shipped.
- Exceptions require an ADR or documented product decision.

---

## Technical Rules

- Follow contracts-first delivery and lane ownership.
- Use secure defaults, structured redacted logs and observable failure paths.
- Keep configuration environment-driven and source controlled where appropriate.

---

## Architecture

AuraEDU Growth is delivered in controlled phases:

1. Foundation: bounded contexts, tenant context, contracts, permissions and observability.
2. Recruitment CRM: consented leads, deduplication, assignment, timeline and source attribution.
3. Public conversion: tenant programme pages, enquiry forms, grounded assistant and knowledge review.
4. Nurturing and admissions: approved email/WhatsApp journeys, application progress and offers.
5. Campaigns and content: budgets, approvals, scheduling, UTM attribution and draft generation.
6. Intelligence: funnel diagnostics, rules-based scoring, forecasts and executive queries with citations.
7. Advanced automation: only after safety, data quality and approval controls are proven.

Current committed foundation is recorded in [ADR 0001](../../decisions/0001-auraedu-growth-bounded-context.md).
Voice agents, autonomous advertising spend, model self-training and uncontrolled public publishing
remain outside the MVP.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- A story affecting future roadmap updates this chapter and its executable evidence in the same review.
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
- [AuraEDU Growth Platform specification](../../product/auraedu-growth-platform-specification.md)
- [Growth bounded-context ADR](../../decisions/0001-auraedu-growth-bounded-context.md)
