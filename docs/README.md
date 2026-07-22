# AuraEDU Engineering Handbook

AuraEDU is an education operating system. This handbook is the living, version-controlled source
of truth for the people and agents who design, build, verify, operate and evolve it.

It is written for:

- engineers and future employees;
- product managers and designers;
- quality, security and operations teams;
- AI coding agents, including Claude Code, Codex, Cursor and Gemini CLI;
- implementation partners and technical reviewers.

## Authority

Use sources in this order:

1. [SUMMARY.md](SUMMARY.md) and the chapter linked from it;
2. accepted architecture decisions in [decisions/](decisions/README.md);
3. published contracts in [../contracts/](../contracts/README.md);
4. the live delivery ledger in [../agent_plan.md](../agent_plan.md);
5. legacy source material, including the former platform specification.

When two sources conflict, the more specific accepted ADR or published contract wins. Open a
documentation issue or story to reconcile the handbook in the same change.

## Chapter contract

Every numbered chapter uses the same reviewable shape:

- Purpose
- Scope
- Principles
- Business Rules
- Technical Rules
- Architecture
- Best Practices
- Examples
- Anti-patterns
- Checklist
- Definition of Done
- References

## How to change the handbook

1. Link the change to an AURA-x.y story or an ADR.
2. Update the affected chapter, contract and implementation together.
3. Mark assumptions and future-state choices explicitly.
4. Never describe a planned capability as shipped.
5. Prefer links to executable contracts and tests over duplicated examples.

## Legacy specification

[AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md](../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
is retained as migration evidence. It is no longer the primary product or engineering authority.

## Product extensions

- [AuraEDU Growth Platform specification](product/auraedu-growth-platform-specification.md) defines the
  recruitment, marketing, admissions-conversion and controlled-AI product domain.
- [ADR 0001](decisions/0001-auraedu-growth-bounded-context.md) records how Growth enters the existing
  monorepo and defines the first enquiry-to-lead delivery slice.
