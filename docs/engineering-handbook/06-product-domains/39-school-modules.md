# Chapter 39: School Modules

## Purpose

School Modules defines the shared rules, boundaries and evidence for this part of the AuraEDU education operating system.

---

## Scope

Product, engineering, operational and governance decisions directly related to school modules.

---

## Principles

- Prefer explicit, reviewable rules over hidden convention.
- Keep tenant boundaries and human accountability visible.
- Use executable contracts and tests as evidence.

---

## Business Rules

- Changes to school modules must name the stakeholder outcome and operational owner.
- Planned capabilities must not be represented as shipped.
- Exceptions require an ADR or documented product decision.

---

## Technical Rules

- Follow contracts-first delivery and lane ownership.
- Use secure defaults, structured redacted logs and observable failure paths.
- Keep configuration environment-driven and source controlled where appropriate.

---

## Architecture

School operations remain the system of record for enrolled learners. AuraEDU Growth is an adjacent
bounded context that owns prospects, recruitment interactions and campaign attribution until an
explicit contract converts an applicant or lead into an enrolled student. Growth reuses identity,
tenant, feature, notification, admissions, payment, analytics and audit services; it does not copy
their data stores.

The first Growth slice is defined by [ADR 0001](../../decisions/0001-auraedu-growth-bounded-context.md)
and the executable [CRM API contract](../../../contracts/openapi/crm.v1.yaml).

Teacher authorization scope is explicit data, not a UI convention. Staff Service owns
teacher-to-class and optional teacher-to-subject assignments and exposes them only through its
public permission boundary and authenticated internal scope contract. Assignment creation and
`staff.assigned.v1` are atomic. Academic Service combines explicit class assignments with the
class-teacher relationship, and downstream attendance, assessment, reporting, AI and portal
flows must consume that resolved scope rather than accept arbitrary class or learner IDs.
School administrators manage the complete staff lifecycle through the People workspace:
create the staff record, optionally link an existing tenant Identity account, activate or
deactivate the person, and then assign teaching scope. The UI never asks an operator to type a
user UUID, and inactive teachers cannot receive new class assignments.

Student administration is an enrolment workflow, not a directory import screen. A school
administrator creates the learner record, may establish an initial class and academic-year
enrolment atomically, and may link or unlink an existing active student Identity account without
typing an opaque identifier. Class movement remains an explicit enrolment operation; generic
student profile updates cannot silently rewrite academic history.

The academic calendar is authoritative operational data. Academic years own teaching terms,
classes and downstream assessment context. Administrators can create and maintain both years and
terms from one workspace, but term dates must remain inside the selected year's date boundary.
Exactly which year is current must remain visible and deliberately configurable; archived years
remain historical records rather than disappearing from reports.

School announcements are published records with an explicit audience, not generic broadcast
text. Administrators choose everyone, students, parents and guardians, or staff; Notification
Service remains authoritative for role-filtered retrieval in web and mobile clients. Operators
must be shown the selected audience and a reviewable message before publication.

---

## Best Practices

- Link decisions to an AURA story or accepted ADR.
- Prefer a single authoritative contract over copied descriptions.
- Review the chapter when implementation or operating policy changes.

---

## Examples

- A story affecting school modules updates this chapter and its executable evidence in the same review.
- A design choice records its trade-offs in an ADR before becoming a platform convention.
- A school administrator assigns a Mathematics teacher to Form 2A once; the teacher then sees
  only Form 2A in web and mobile attendance, score-entry and reporting workflows.
- A registrar creates a learner and the learner's first enrolment together, then links the portal
  account selected from active tenant Identity users.
- A school administrator defines the 2026/2027 year before adding its terms and classes, and the
  portal constrains each term to the owning year's calendar window.
- An administrator publishes a reopening update to parents and guardians; student and staff
  inboxes cannot retrieve that targeted record.

---

## Anti-patterns

- Duplicating a contract as prose and allowing the copies to drift.
- Claiming completion without tests, monitoring or an owner.
- Creating tenant-specific code paths.
- Treating a teacher role as permission to read every class in the tenant.
- Accepting raw Identity UUIDs from school operators or using a student profile patch to rewrite
  enrolment history.
- Showing academic years without their terms, current-cycle state or operational actions.
- Treating a targeted announcement as an unscoped tenant-wide message in a client.

---

## Checklist

- Is the outcome and owner clear?
- Are tenant, permission and feature boundaries covered?
- Are contracts, tests and runbooks updated?
- Do teacher workflows resolve explicit class scope at the service boundary and fail closed when
  Staff Service is unavailable?
- Does student account linking select only active tenant-owned Identity users and support a
  deliberate unlink operation?
- Do academic terms retain an immutable owning year and respect that year's date boundary?
- Is an announcement audience explicit at publication and enforced again during retrieval?
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
