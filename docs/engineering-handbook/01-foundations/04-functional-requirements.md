# Chapter 04: Functional Requirements

## Purpose

Define the capabilities AuraEDU must provide as a complete education operating system.

---

## Scope

Current MVP modules, planned school domains, platform capabilities and responsible AI functions.

---

## Principles

- Model capabilities around school outcomes and bounded domains.
- Keep each service and module independently deployable and feature-controlled.
- Deliver web and mobile parity for teacher, parent and student workflows.

---

## Business Rules

- Core domains include admissions, students, guardians, staff, academics, attendance, assessments, report cards, fees, payments, notifications, websites, files, analytics, audit and billing.
- Extended school domains include library, hostel, transport, alumni, inventory, procurement, HR and payroll.
- AI domains include recommendations, prediction, career guidance, approved feature data and model governance.
- Every module defines roles, permissions, feature eligibility, events, reports and retention rules.

---

## Technical Rules

- Published OpenAPI and event schemas are the executable functional contract.
- Each service owns its database and exposes behaviour through APIs or events.
- The live implementation sequence remains in agent_plan.md.

---

## Architecture

Capabilities are grouped into people and academics, teaching and learning, finance and communication, insight and AI, public experiences and platform control.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- Attendance supports class and subject registers plus downstream events.
- Fees supports structures, invoices, balances and receipts; payment handles provider transactions and reconciliation.
- Website management publishes tenant-branded public content without changing the company marketing app.

---

## Anti-patterns

- One service becoming a generic school database.
- Building a UI before the owning contract exists.
- Calling a backlog module complete because a menu item exists.

---

## Checklist

- Are actors and permissions defined?
- Is the feature key defined?
- Are API and event contracts published?
- Are tenant isolation and off-state tests present?
- Is platform parity explicit?

---

## Definition of Done

- Every module has an owner, contract, feature policy and Definition of Done.
- Implemented and planned capabilities are visibly distinguished.
- The chapter and live backlog are reconciled each release.

---

## References

- [AuraEDU Engineering Handbook](../../README.md)
- [Agent execution plan](../../../agent_plan.md)
- [Architecture and product source material](../../../AuraEDU_Microservices_Multi_Tenant_SaaS_Specification.md)
- [Design system](../../../DESIGN_SYSTEM.md)
- [Contracts](../../../contracts/README.md)
