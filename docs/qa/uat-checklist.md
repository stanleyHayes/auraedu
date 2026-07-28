# UAT & pre-launch regression checklist — AuraEDU

This is the gate that must pass before a release candidate is promoted to production. It has
two halves: a **regression checklist** QA executes against staging, and a **UAT sign-off** a
school-facing owner executes against the same build. Every item is phrased as observable
behavior with an exact expected result — check the box only when the expected result was
seen, not assumed.

How to use:

- Execute top to bottom against one release candidate. Record the candidate commit SHA in the
  sign-off block.
- Any FAIL stops the run. Log it with [findings-template.md](findings-template.md), fix,
  restart the affected section, and re-run the smoke suite (§16) at the end.
- Route paths below are the web portal (`apps/web`). API paths are from `contracts/openapi/`.

---

## 1. Environment preamble

Complete this section before any test item. If any preamble item fails, the run is invalid.

### 1.1 Migration state

- [ ] `make migrate-check` passes. **Expected:** migration inventory, sequencing and markers
      validate with no drift.
- [ ] `make migrate` completes against the target database. **Expected:** every service's
      Goose ledger reports the latest version; no `pending` migrations remain.
- [ ] RLS inventory is intact. **Expected:** every tenant-owned table has `ENABLE RLS`,
      `FORCE RLS` and an `app.tenant_id` policy (CI enforces this via the isolation gate;
      confirm the deployed database matches — see AURA-50.1).

### 1.2 Required environment variables

Verify these are set for the environment under test (names per `deploy/docker-compose.yml`):

- [ ] `PAYMENTS_PROVIDER=paystack` for any money-rails test. **Expected:** the payment service
      starts; with the default `mock` provider, payment initiation is deterministic and no
      real charge occurs — money items in §9 and §15 are invalid under `mock`.
- [ ] `PAYSTACK_SECRET_KEY` and `PAYSTACK_WEBHOOK_SECRET` are set. **Expected:** staging uses
      **test-mode keys (`sk_test_…`) only**. Live keys (`sk_live_…`) in staging is an
      automatic run-stopper. Note the inverse gate: production configuration rejects
      non-live keys and mock mode outright (AURA-17.13), so a staging build cannot be
      promoted unchanged without key rotation.
- [ ] `RESEND_API_KEY`, `RESEND_WEBHOOK_SECRET`, `RESEND_FROM_EMAIL` are set.
      **Expected:** transactional email sends and Resend delivery webhooks are accepted.
- [ ] `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, and `TWILIO_SMS_FROM` or
      `TWILIO_MESSAGING_SERVICE_SID` are set. **Expected:** SMS/WhatsApp delivery attempts
      reach Twilio; an unconfigured channel must surface as `unconfigured` in delivery logs,
      never silently drop (see §12).

### 1.3 Seeded demo tenants and credentials

- [ ] `make seed` has been run. **Expected:** two demo tenants exist — `upshs` (Union
      Preparatory SHS, `upshs.auraedu.com`) and `aboom` (Aboom Senior High,
      `aboom.auraedu.com`) — each on the `starter` plan, plus the platform super admin.
- [ ] Seeded credentials are available. **Expected:** the seed run prints a credential table
      (email / role / password). Canonical accounts: `superadmin@auraedu.com`
      (platform_super_admin), `admin@upshs.edu` (school_admin, tenant `upshs`),
      `admin@aboom.edu` (school_admin, tenant `aboom`). The password defaults to
      `Password123`, overridable via `SEED_PASSWORD` at seed time — source of truth is
      `tools/seed/main.go`, not this document. Rotate seeded passwords before any external
      UAT participant touches the environment.

---

## 2. School onboarding & first-admin invite

- [ ] Platform super admin submits a new school onboarding request and approves it from
      `/superadmin/onboarding`. **Expected:** the request moves pending → approved; the
      tenant is created; the approval is recorded in the audit log.
- [ ] The first-admin invite email is delivered. **Expected:** the invite arrives via Resend
      with a one-time acceptance link; delivery appears in notification delivery logs.
- [ ] The invitee opens the link and completes `/accept-invite`. **Expected:** password set
      succeeds; the invite link cannot be replayed (second use is rejected); the new admin
      can log in at `/login` and lands on `/admin`.
- [ ] The same onboarding request cannot be approved twice. **Expected:** a repeat approval
      returns a conflict/validation error, not a duplicate tenant.
- [ ] Rejected onboarding does not activate. **Expected:** rejecting a request leaves no
      active tenant and no usable invite.

## 3. People management (students, staff, guardians)

- [ ] School admin creates a student at `/admin/students`. **Expected:** the student appears
      in the list with the assigned class; an enrollment record is written atomically
      (one record per learner per academic year, AURA-10.11).
- [ ] School admin creates staff at `/admin/staff` and manages login users at `/admin/users`.
      **Expected:** invite, role reassignment and deactivation work; grant-validation errors
      surface in the UI rather than failing silently.
- [ ] CSV bulk import of students succeeds with a valid file. **Expected:** the import
      summary reports created/updated/skipped counts; re-importing the same file is
      idempotent (no duplicate students).
- [ ] CSV bulk import with malformed rows fails safely. **Expected:** row-level errors are
      reported with line numbers; valid rows in the same file are not rolled back by invalid
      ones (or the whole import is rejected atomically — either is acceptable, partial silent
      import is not).
- [ ] Guardian linkage works. **Expected:** a guardian linked to a student can log in to the
      parent portal and sees only their own children at `/parent/children`.

## 4. Academic structure

- [ ] Admin creates an academic year with terms at `/admin/academic-years`. **Expected:**
      exactly one year can be active/current; term dates are validated within the year.
- [ ] Admin creates classes and subjects at `/admin/classes` and `/admin/subjects`.
      **Expected:** classes reference a valid academic year; subjects can be assigned to
      classes and teachers.
- [ ] Programmes are managed at `/admin/programmes`. **Expected:** programmes created here
      become visible on the public website at `/{tenant}/programmes` (see §13).
- [ ] Grading scales are tenant-owned and non-overlapping. **Expected:** creating an
      overlapping 0–100 range is rejected (AURA-12.9).

## 5. Attendance

- [ ] Teacher marks daily attendance at `/teacher/attendance` for an assigned class.
      **Expected:** the roster loads for that class; submit succeeds; re-submitting the same
      day restores and updates the existing record instead of duplicating it (idempotent
      daily attendance).
- [ ] Teacher cannot mark attendance for an unassigned class. **Expected:** the class does
      not appear in their picker; a direct API attempt is denied.
- [ ] Admin reviews and corrects attendance at `/admin/attendance`. **Expected:** the admin
      sees school-wide attendance and can correct a record; the correction is audit-logged
      with actor and timestamp.
- [ ] Parent sees their child's attendance at `/parent/attendance`. **Expected:** only their
      own children's records, read-only.

## 6. Assessments & score entry

- [ ] Teacher creates an assessment and enters scores at `/teacher/scores`. **Expected:**
      only assessments/classes assigned to that teacher are offered; scores save per student.
- [ ] Role scoping is enforced. **Expected:** a teacher sees **only their assigned classes**
      in pickers and lists; a direct API call for another class's scores is denied (403 or
      non-enumerating 404).
- [ ] Admin oversight at `/admin/assessments` is school-wide. **Expected:** the admin sees
      all assessments and score-entry progress for the school.
- [ ] Published results reach learners. **Expected:** published scores appear at
      `/student/results` and `/parent/results`; unpublished scores never do.

## 7. CBT exam lifecycle

- [ ] Staff authors a CBT exam at `/admin/cbt` (questions, options, correct answers,
      duration) and publishes it. **Expected:** a draft exam is invisible to students;
      publishing makes it available to the target class within the exam window.
- [ ] Student takes the exam at `/student/cbt-exams`. **Expected:** the exam list shows
      active sessions only; questions are served without correct answers; a second attempt
      for the same exam is rejected (duplicate-attempt prevention, AURA-24.6).
- [ ] Submission is graded server-side. **Expected:** final answers are accepted exactly
      once; grading happens on the server; the student cannot resubmit or mutate answers
      after submission.
- [ ] Admin oversight is read-only. **Expected:** `/admin/cbt` exposes exam and submission
      oversight without letting the admin alter student answers (AURA-46.13).
- [ ] Out-of-scope attempts are hidden. **Expected:** a student probing another student's
      attempt ID receives a non-enumerating 404.

## 8. Report cards

- [ ] Admin generates a report card from `/admin/reports`. **Expected:**
      `POST /report-cards/{id}/generate` returns `202`; the PDF is produced asynchronously by
      the worker and becomes downloadable when ready (durable queue, AURA-15.9).
- [ ] Admin publishes the report card. **Expected:** published state is committed with a
      tenant-isolated outbox event; only published cards are visible to learners and parents.
- [ ] Student views at `/student/report-card`; parent views at `/parent/reports`.
      **Expected:** each sees only their own / their children's published cards.
- [ ] PDF download works and is authorization-proxied. **Expected:** download via
      `/report-cards/{id}/download` (web proxy route `/api/reports/[report_card_id]`)
      streams a valid PDF; an unauthenticated or out-of-scope request is denied; storage
      keys never appear in responses.
- [ ] Transcripts derive from published cards. **Expected:**
      `GET /transcripts/{student_id}` reflects published/archived cards only (AURA-15.11).

## 9. Fees, invoices & Paystack payments

- [ ] Admin defines fee structures and issues invoices at `/admin/fees`. **Expected:**
      invoice creation emits the fee-assigned and invoice-created lifecycle; balances are
      currency-separated per learner (AURA-16.9/16.10).
- [ ] Parent sees invoices at `/parent/fees` and initiates payment at `/parent/payments`.
      **Expected:** only invoices for their own children are listed; initiation returns a
      validated **HTTPS** Paystack checkout URL; on provider or URL failure the payment rolls
      back to `pending`, never stuck in `processing` (AURA-17.9).
- [ ] Paystack webhook settles the payment. **Expected:** the signed webhook moves the
      payment to succeeded, writes one immutable receipt, and the invoice/balance reflects
      the payment exactly once; `payment.received.v1` is committed atomically with the
      ledger change (transactional outbox, AURA-17.11).
- [ ] Webhook signature verification is enforced. **Expected:** a webhook with an invalid
      signature is rejected and does not change payment state (with
      `PAYSTACK_WEBHOOK_SECRET` set — unsigned acceptance is dev-mode only).
- [ ] Under-payment is handled. **Expected:** a partial payment leaves the invoice in a
      partial/paid-partial state with the remaining balance; it is not marked fully paid and
      no overpayment is fabricated (AURA-16.11: partial/paid transition, explicit
      overpayment).
- [ ] Timeout/abandoned checkout recovery does not double-charge. **Expected:** a parent who
      abandons checkout and re-initiates gets a single settled payment; manual verify
      (`payment verify`) reconciles the same provider reference idempotently — one receipt,
      one balance movement.
- [ ] Receipts are immutable. **Expected:** `GET /receipts/{receipt_id}` returns the same
      receipt after replay, refund discussion, or admin review; receipts are never edited in
      place.

## 10. Admissions pipeline

- [ ] A member of the public submits an application from the school's public site (see §13).
      **Expected:** the application is created in draft/submitted state without any login;
      programmes listed at `/{tenant}/programmes` match `/admin/programmes`.
- [ ] Staff reviews at `/admin/admissions`. **Expected:** review transitions the application
      (`/applications/{id}/review`); documents attached via
      `/applications/{id}/documents` are visible to reviewers.
- [ ] Staff issues an offer. **Expected:** `/applications/{id}/offer` records the offer and
      notifies the applicant.
- [ ] Applicant accepts the offer in the applicant portal (`/applicant`). **Expected:**
      `/applications/{id}/offer/accept` transitions to accepted; double-accept is rejected;
      the admitted applicant can be converted to an enrolled student.
- [ ] Cross-school privacy holds. **Expected:** applicants see only their own application;
      staff of school A cannot enumerate school B's applications.

## 11. Communication journeys

- [ ] Admin authors a journey at `/admin/journeys` and activates it. **Expected:**
      `/communication-journeys/{id}/activate` starts enrolment; pause and archive
      transitions behave; stats at `/communication-journeys/{id}/stats` update after sends.
- [ ] Journey messages are actually delivered. **Expected:** recipients receive the first
      step; each send appears in delivery logs with a terminal state.
- [ ] The unsubscribe link works. **Expected:** the link in a journey email opens
      `/unsubscribe`, confirms without requiring login, posts to
      `/email-preferences/unsubscribe`, and the unsubscribed address receives **no further
      journey email** on the next step while transactional mail (e.g. receipts) still
      arrives.

## 12. Notification delivery visibility

- [ ] Admin opens delivery logs at `/admin/delivery`. **Expected:** every notification shows
      channel (email / SMS / WhatsApp / push) and an unmistakable outcome state — delivered,
      failed, or unconfigured; failed states include the provider error.
- [ ] Provider webhooks update state. **Expected:** Resend (`/webhooks/resend`) and Twilio
      (`/webhooks/twilio`) callbacks move a message from sent to delivered/bounced/failed.
- [ ] Unconfigured channels fail loudly. **Expected:** requesting WhatsApp without Twilio
      configuration records `unconfigured` in the log and fires no silent drop (this state
      also drives the `AuraEDUNotificationDeliveryFailures` alert).
- [ ] Parents can review their notifications at `/parent/notifications`. **Expected:** only
      their own messages, with per-channel preferences respected.

## 13. Public school website & admissions assistant

- [ ] The public site renders at `/{tenant}` (e.g. `/upshs`) with pages at `/{tenant}/[slug]`
      and programmes at `/{tenant}/programmes`. **Expected:** content is tenant-scoped —
      school A's site never shows school B's content; unpublished pages 404.
- [ ] Website content is editable at `/admin/website`. **Expected:** an admin edit appears on
      the public site after publish.
- [ ] The admissions assistant asks for consent. **Expected:** before processing a chat
      message (`POST /public/assistant/messages`), the assistant obtains and records consent
      for personal-data use; no personal data is retained without it.
- [ ] The assistant admits uncertainty. **Expected:** an out-of-scope question (e.g. "what
      are next year's fees exactly?") gets an honest "I don't know / please contact the
      school" answer, not a fabricated one.
- [ ] Handoff works. **Expected:** on request or repeated uncertainty, the assistant hands
      off to a human channel (school contact / admissions enquiry), and the handoff is
      visible to staff (leads/enquiries at `/admin/leads`).

## 14. AI oversight

- [ ] A pending AI recommendation is invisible to the student. **Expected:** a generated
      recommendation in `pending` state does **not** appear at `/student/recommendations` or
      `/parent/guidance`; it appears only after a teacher/admin approves it
      (`ai.action.approve`, AURA-30.6/30.7).
- [ ] Approve / reject / override work from the oversight surface. **Expected:**
      `/admin/ai` (and the teacher review workspace) show pending-inclusive lists; teacher
      actions are limited to assigned learners; override records the human decision.
- [ ] Predictions and career guidance follow the same gate. **Expected:** pending
      predictions and career guidance are hidden from learners and parents until approved
      (AURA-31.2, AURA-32.2/48.5).
- [ ] AI-disabled tenants see nothing. **Expected:** with the AI feature flag off for a
      tenant, AI surfaces are hidden in the portal and the API returns `403
      feature_disabled` (fail-closed entitlements, AURA-51.x).

## 15. Platform super-admin console

- [ ] Tenant drill-down works from `/superadmin/tenants`. **Expected:** the tabbed tenant
      detail (students, staff, finance, attendance, audit, delivery) loads real per-school
      data pinned via `X-Tenant-Code`; counts are honest (capped where capped).
- [ ] Cross-tenant audit explorer filters work at `/superadmin/audit-logs`. **Expected:** an
      all-tenants option exists; filters `event_type`, `actor_id`, `source_service`,
      `from`, `to` narrow results; invalid filter values return a validated 422
      (AURA-46.13).
- [ ] Onboarding approval works (covered in §2) from `/superadmin/onboarding`.
- [ ] Billing plan CRUD works at `/superadmin/billing-plans`. **Expected:** create, update
      and retire plans; subscription assignment at `/superadmin/subscriptions` changes the
      tenant's plan.
- [ ] Feature-flag override requires a reason at `/superadmin/flags`. **Expected:** setting
      a per-tenant override records the reason and actor; the override takes effect (module
      hidden/shown) without a deploy; clearing the override restores the plan default.
- [ ] System health is truthful at `/superadmin/system-health`. **Expected:** the
      gateway-owned `/api/v1/platform/health` report shows healthy / degraded / unreachable
      per service, never a fabricated "all good" (AURA-8.2).

## 16. Security & multi-tenancy

- [ ] Cross-tenant access is denied. **Expected:** logged in as `admin@upshs.edu`, requesting
      an `aboom` resource by ID returns a non-enumerating **404**; a token/header tenant
      mismatch returns **403**; denial bodies disclose nothing about the other tenant
      (AURA-50.2 shape).
- [ ] RLS fails closed at the database. **Expected:** with no `app.tenant_id` set, tenant
      tables return zero rows; there is no code path that reads tenant data without setting
      tenant context.
- [ ] Client-supplied actor headers are stripped. **Expected:** sending `X-Actor-*` headers
      through the gateway has no effect; identity comes only from the signed JWT
      (AURA-53.1).
- [ ] Only platform super admins can pin `X-Tenant-Code`. **Expected:** a school admin
      sending `X-Tenant-Code: aboom` cannot escape their tenant; the resolved tenant must
      match the JWT tenant claim; tenantless non-platform actors fail closed.
- [ ] Login lockout works. **Expected:** repeated failed logins at `/login` lock or throttle
      the account with a clear retry message (and drive `AuraEDUSustainedFailedLogins` at
      platform level); a legitimate user recovers via `/forgot-password` →
      `/reset-password`.
- [ ] MFA is enforced for privileged roles. **Expected:** platform super admin and school
      admin accounts complete MFA enrolment/challenge; a privileged session without MFA
      cannot reach `/superadmin` or sensitive `/admin` actions.
- [ ] Refresh tokens rotate. **Expected:** using a refresh token issues a new pair and
      invalidates the old one; replaying the old refresh token fails and revokes the session
      lineage; sign-out revokes the server-side refresh session.
- [ ] Disabled modules are enforced everywhere. **Expected:** a feature-disabled module is
      hidden from navigation, denied on direct portal routes, rejected by the gateway with
      `403 feature_disabled`, and produces no background work (AURA-51.1).

## 17. Money rails — cross-cutting

Run these once per release, independent of the feature sections above.

- [ ] Webhook delivery is idempotent. **Expected:** replaying the exact same Paystack
      `charge.success` webhook (same provider reference) three times produces **one**
      payment transition, **one** immutable transaction, **one** receipt; the deduplication
      check (`HasProcessedReference`,
      `apps/payment-service/internal/adapters/postgres/repository.go`) short-circuits
      replays.
- [ ] Amount reconciliation is exact. **Expected:** for every settled payment, payment
      amount = sum of its transactions = the amount credited on the invoice(s); balances
      update in the payment's currency only; any mismatch is recorded as explicit
      overpayment, never silently absorbed.
- [ ] No double settlement on consumer replay. **Expected:** redelivering
      `payment.received.v1` to the Fees service reconciles exactly once (transaction /
      advisory lock), emits `invoice.updated.v1` / `invoice.paid.v1` at most once per
      transition (AURA-16.11).
- [ ] Outbox atomicity holds. **Expected:** payment status, ledger transaction and the
      domain event commit in one database transaction — there is no state where the payment
      succeeded but no event exists, or an event exists for a rolled-back payment
      (AURA-17.11/17.12).
- [ ] Failure events are truthful. **Expected:** a failed charge emits `payment.failed.v1`,
      leaves the invoice unpaid, and does not touch balances.
- [ ] End-to-end money smoke in Paystack **test mode**. **Expected:** initiate → test
      charge → webhook → receipt → invoice paid, with the Paystack dashboard test
      transaction matching the AuraEDU receipt to the pesewa.

## 18. Fast regression smoke (run last, ~10 items, ≤ 30 minutes)

- [ ] 1. Login as `admin@upshs.edu` lands on `/admin`; login as `superadmin@auraedu.com`
      lands on `/superadmin`.
- [ ] 2. Create one student; the student appears in `/admin/students` immediately.
- [ ] 3. Teacher marks today's attendance for an assigned class; re-submit updates, not
      duplicates.
- [ ] 4. Teacher enters one score; it is invisible to the student until published.
- [ ] 5. Publish one report card; the parent sees it and downloads the PDF.
- [ ] 6. Initiate one Paystack test payment; webhook settles it; exactly one receipt.
- [ ] 7. Replay that webhook; nothing changes (§17).
- [ ] 8. Cross-tenant probe: upshs admin fetching an aboom invoice ID gets 404.
- [ ] 9. Activate a one-step journey to a test address; unsubscribe link stops further sends.
- [ ] 10. Approve a pending AI recommendation; it becomes visible to the student only after
      approval.

## 19. Sign-off

| Field | Value |
|---|---|
| Release candidate commit SHA | |
| Environment (staging URL / DB) | |
| Run date(s) | |
| QA lead (name / signature) | |
| UAT owner (name / signature) | |
| Overall result | PASS / FAIL |
| Open defects (finding IDs + links) | |
| Deferred items (with owner + rationale) | |
| Paystack key mode used | test / live (must be **test** for staging) |
| Notes | |

**Rule:** PASS requires every checkbox in §§2–17 checked or formally deferred in this table,
plus a clean §18 smoke on the final candidate.
