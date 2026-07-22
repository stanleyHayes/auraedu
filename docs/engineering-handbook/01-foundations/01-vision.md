# Chapter 01: Vision

## Purpose

Explain why AuraEDU exists: to help education institutions account for learners, time, money, decisions and trust without increasing the burden on educators.

---

## Scope

The education problems AuraEDU chooses to solve; the people it serves; the role of data and AI; the boundaries of the mission.

---

## Principles

- Education outcomes come before software adoption.
- Schools are institutions, not generic small businesses.
- Teachers need leverage, not another reporting layer.
- Students and families deserve timely, understandable information.
- AI assists accountable people and never replaces educators.
- Useful data must remain contextual, consented and tenant-isolated.

---

## Business Rules

- AuraEDU serves many schools on one platform while each school retains its identity and data boundary.
- The platform must work for resource-constrained schools as well as larger institutions.
- A capability that cannot be explained to school leaders should not be imposed on them.

---

## Technical Rules

- Every design decision must name the educational or operational outcome it supports.
- No school-specific code forks; variation is configuration, branding and feature policy.
- No AI output is final academic truth.

---

## Architecture

The mission is realised through a multi-tenant platform, role-specific web and mobile experiences, independently owned services and governed AI capabilities.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- A teacher takes attendance once; the same trusted event can inform the parent portal and analytics without duplicate entry.
- A school enables fees when ready without deploying a different product.
- A risk signal includes evidence and confidence, then waits for human review.

---

## Anti-patterns

- Building features because competitors list them.
- Automating a judgement that requires educator accountability.
- Using engagement metrics as a substitute for learning outcomes.

---

## Checklist

- Does this reduce friction for a real education stakeholder?
- Does it preserve human accountability?
- Can a school understand and control it?
- Does it respect tenant, privacy and accessibility boundaries?

---

## Definition of Done

- The chapter is referenced by product principles and roadmap decisions.
- Every new epic states the vision outcome it advances.
- Conflicting product ideas are resolved through an ADR or documented product decision.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
