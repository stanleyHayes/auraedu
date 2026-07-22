# Chapter 03: Product Principles

## Purpose

Establish immutable product rules that guide prioritisation, design and engineering decisions.

---

## Scope

Principles that apply to every module, integration, interface and AI capability.

---

## Principles

- Every feature must improve learning, institutional reliability or stakeholder clarity.
- No feature should increase teacher workload without removing a greater burden.
- AI assists teachers and never replaces them.
- One codebase serves many distinct schools.
- Secure, accessible and explainable defaults are product features.
- Configuration beats custom forks.

---

## Business Rules

- Product proposals must state the stakeholder outcome and measure of success.
- A module may launch only when its off-state and downgrade state are designed.
- Administrative convenience cannot override student privacy or teacher accountability.

---

## Technical Rules

- Feature gates exist in frontend, API and workers.
- Protected actions enforce identity, tenant, permission, feature and resource ownership checks.
- Accessibility and reduced-motion support are release requirements.

---

## Architecture

Principles flow into story acceptance criteria, design-system rules, contracts, tests and release gates.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- An AI recommendation remains pending until approved.
- A disabled module is absent from navigation and blocked at the API.
- A tenant brand changes accent tokens, not component code.

---

## Anti-patterns

- Dark patterns that push schools into upgrades.
- Generic dashboards with no role outcome.
- An AI score with no explanation or review path.

---

## Checklist

- Which principle does the change advance?
- What workload does it remove?
- What happens when the feature is disabled?
- Who remains accountable?

---

## Definition of Done

- The story links to at least one product principle.
- Acceptance tests prove the relevant principle.
- Exceptions have an accepted ADR and expiry or review condition.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
