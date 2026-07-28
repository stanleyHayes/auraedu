# AuraEDU user guide

This guide is for the people who use AuraEDU every day: school staff, teachers, parents,
students, applicants, and the AuraEDU platform team. It is organized by audience, with four
cross-cutting sections at the end that apply to everyone.

> **Screenshot placeholders.** Sections marked 📷 need product screenshots inserted
> post-launch. Capture them from the production web portal and drop them in
> `docs/assets/user-guide/` with the filename noted in the placeholder.

---

## 1. School administrators

Your home is the admin portal at `/admin` after sign-in.

### 1.1 Getting your school set up 📷 `admin-dashboard.png`

1. Your school's onboarding is approved by the AuraEDU platform team; your first admin
   account arrives by email invite. Accept it, set your password, and sign in (see §7).
2. Create your **academic structure** first — nothing else works without it:
   - Academic year and terms: `/admin/academic-years`
   - Classes: `/admin/classes` · Subjects: `/admin/subjects` · Programmes: `/admin/programmes`
3. Add your people:
   - Staff and login users (invite, role changes, deactivation): `/admin/staff`, `/admin/users`
   - Students, one by one or by **CSV bulk import**: `/admin/students`. Download the CSV
     template from the import dialog; the import summary tells you exactly which rows were
     created, updated or skipped and why.
4. Assign teachers to classes and subjects so their attendance and score screens show the
   right rosters.

### 1.2 Day-to-day operations

- **Attendance oversight and corrections:** `/admin/attendance`. Teachers mark; you correct.
  Every correction is recorded with who and when.
- **Assessments and score progress:** `/admin/assessments` — see every assessment and how
  far score entry has gotten per class.
- **CBT exams:** `/admin/cbt` — author exams, publish them to a class, and review
  submissions (oversight is read-only; student answers cannot be edited by staff).
- **Report cards:** `/admin/reports` — generate (this runs in the background; the PDF
  appears when ready), review, then **publish**. Only published cards are visible to
  students and parents.
- **Audit log:** `/admin/audit` — every significant action in your school, filterable.

### 1.3 Fees and payments

- Define **fee structures** and issue **invoices** at `/admin/fees`. Balances are tracked
  per learner, per currency.
- Track incoming money at `/admin/payments`, including the payment provider webhook audit
  trail. See §8 for how settlement works and what to tell parents.

### 1.4 Admissions, website and communication

- **Admissions pipeline:** `/admin/admissions` — review public applications, issue offers,
  track acceptances.
- **Your public website:** edit pages at `/admin/website`; the public sees them at
  `/{your-school-code}` and `/{your-school-code}/programmes`.
- **Communication journeys:** `/admin/journeys` — author multi-step email/SMS journeys,
  activate, pause, and read per-journey stats.
- **Delivery logs:** `/admin/delivery` — every message with its channel and outcome
  (delivered / failed / unconfigured). When a parent says "I never got it," start here.
- **AI oversight:** `/admin/ai` — review, approve, reject or override AI recommendations,
  predictions and career guidance. **Nothing reaches a student or parent until a human
  approves it.**

---

## 2. Teachers

Your home is `/teacher`.

- **Attendance:** `/teacher/attendance` — pick one of your assigned classes and mark today's
  register. Re-submitting the same day updates the register; you cannot create duplicates.
- **Scores:** `/teacher/scores` — create assessments and enter scores for **your assigned
  classes only**. If a class is missing, ask your admin to check the assignment — the system
  will not let you see other teachers' classes.
- **Your classes and rosters:** `/teacher/classes`.
- **Reports and analytics:** `/teacher/reports`, `/teacher/analytics`.
- **AI review:** when the school uses AI recommendations or career guidance, you review
  suggestions for your assigned learners and approve, reject or override them before
  learners ever see them. 📷 `teacher-review.png`

---

## 3. Parents and guardians

Your home is `/parent`. You see only your own children.

- **Children:** `/parent/children` — linked learners.
- **Attendance:** `/parent/attendance` — daily records per child.
- **Results and report cards:** `/parent/results`, `/parent/reports` — published scores and
  report cards, with PDF download.
- **Fees and payments:** `/parent/fees` shows invoices; `/parent/payments` starts a secure
  Paystack checkout. See §8 for settlement timing, partial payments and refunds.
- **Guidance:** `/parent/guidance` — approved career guidance and recommendations.
- **Notifications:** `/parent/notifications` — your message history and preferences (§9).

The mobile app offers the same parent workflows with push notifications.

---

## 4. Students

Your home is `/student`.

- **Timetable:** `/student/timetable` — your class's weekly lessons.
- **Assignments:** `/student/assignments`.
- **CBT exams:** `/student/cbt-exams` — exams appear here only while the exam window is
  active. You get **one attempt**; answers are final once submitted and graded by the
  system, not by guessable answer keys.
- **Results and report cards:** `/student/results`, `/student/report-card` — only after
  your school publishes them.
- **Recommendations:** `/student/recommendations` — AI suggestions appear here only after a
  teacher has approved them.

---

## 5. Applicants

You do not need an account to start: apply from the school's public website
(`/{school-code}` → admissions/apply). You can then track your application, upload
documents, and accept an offer in the applicant portal at `/applicant`. 📷
`applicant-portal.png`

The website's **admissions assistant** chat can answer questions about programmes and the
process. It will always ask your consent before using personal details, will say so plainly
when it does not know something, and will hand you to a human at the school when you ask or
when it cannot help.

---

## 6. Platform operators (AuraEDU team)

Your home is `/superadmin`.

- **Tenants:** `/superadmin/tenants` — per-school drill-down (students, staff, finance,
  attendance, audit, delivery) scoped with `X-Tenant-Code`.
- **Onboarding:** `/superadmin/onboarding` — approve or reject new school requests.
- **Billing:** `/superadmin/billing-plans`, `/superadmin/subscriptions` — plan CRUD and
  tenant subscription management.
- **Feature flags:** `/superadmin/flags` — per-school overrides; a reason is mandatory and
  recorded.
- **Audit explorer:** `/superadmin/audit-logs` — cross-tenant search with event type, actor,
  source service and time-range filters.
- **System health:** `/superadmin/system-health` — live per-service health across the
  platform.
- Incidents: follow
  [the incident-response runbook](engineering-handbook/04-operations/runbooks/incident-response.md).

---

## 7. Sign-in & security — for everyone

- **Workspace code.** AuraEDU is multi-school. At sign-in you enter your school's
  **workspace code** (e.g. `upshs`) plus your email and password. The code selects your
  school; your account only ever sees that school's data. The mobile app remembers your
  school after first sign-in.
- **MFA.** Privileged roles (school admins, platform staff) must enrol a second factor.
  Complete enrolment at first sign-in when prompted; keep your recovery codes somewhere
  safe.
- **Locked out?** Too many failed attempts lock or throttle the account. Use
  **Forgot password** (`/forgot-password`) → the reset email → set a new password
  (`/reset-password`). If the reset email does not arrive, check spam, then ask your school
  admin (or, for school admins, the AuraEDU team) — never share your password with anyone
  "helping" you.
- **Sessions.** Signing in issues short-lived tokens that rotate automatically; signing out
  ends the session on the server, not just on your device. Sign out on shared computers.

## 8. Payments & money, in plain language

- **How paying fees works.** From `/parent/payments` (or the mobile app) you are sent to a
  secure **Paystack** checkout page over HTTPS. AuraEDU never sees or stores your card or
  mobile-money PIN — that stays with Paystack.
- **When the invoice shows "paid."** After you pay, Paystack notifies AuraEDU (a "webhook")
  and the invoice updates automatically — usually within seconds. If you close the browser
  right after paying, nothing is lost: the notification still arrives, and the school can
  also verify the payment manually. You will **never be charged twice** for starting the
  payment again after a timeout; a repeated notification settles the invoice once and is
  otherwise ignored.
- **Paying part of an invoice.** A partial payment is recorded as a partial payment: the
  remaining balance stays on the invoice and the receipt shows exactly what you paid. If you
  ever overpay, the excess is recorded explicitly and visible to the school — it does not
  vanish.
- **Receipts.** Every successful payment produces an immutable receipt available from the
  fees screen. It cannot be edited later — this is deliberate, so a receipt is always proof.
- **Refunds.** Refunds are initiated by the school through Paystack, not self-serve. Mobile
  money refunds typically return within 1–3 business days; card refunds can take 5–10
  business days depending on the bank. If a refund seems stuck, give the school your
  receipt's payment reference — it identifies the transaction end to end.

## 9. Notifications & unsubscribing

- **Channels.** AuraEDU sends email (receipts, invites, report-card notices, journeys), SMS
  and WhatsApp (urgent school messages, where configured), and mobile push (if you use the
  app and allow it).
- **Marketing vs essential.** Communication journeys and announcements are optional.
  **Transactional** messages — payment receipts, password resets, invite emails — are part
  of the service and cannot be unsubscribed from.
- **Unsubscribing.** Every journey email ends with an unsubscribe link. Clicking it works
  **without signing in** and stops that school's optional email to your address. Parents can
  also adjust channel preferences under `/parent/notifications`; mobile push is controlled
  in the app and your device settings.

## 10. FAQ & troubleshooting

- **"I signed in but see an empty portal / the wrong school."** You probably used the wrong
  workspace code. Sign out and sign in with the code in your invite email.
- **"I paid but the invoice still shows unpaid."** Wait two minutes and refresh — webhook
  settlement is near-instant but not synchronous. If it persists, give the school your
  Paystack reference; staff can verify the payment manually from `/admin/payments`.
- **"My report card / results are missing."** They are visible only after the school
  **publishes** them. Ask the school, not AuraEDU support.
- **"The AI recommendation my child mentioned isn't showing."** Learners only see AI
  recommendations after a teacher approves them; a pending one is invisible by design.
- **"The CBT exam says I already attempted it."** One attempt per student per exam. If you
  were disconnected mid-exam, contact your teacher — staff can review the submission record.
- **"I'm not receiving emails."** Check spam, then ask the school to look at
  `/admin/delivery` — it shows whether each message was delivered, failed, or the channel
  was never configured.
- **"The public website shows outdated information."** School staff publish website changes
  from `/admin/website`; report it to the school office.
- **Still stuck?** Parents and students → your school office first. School staff → AuraEDU
  support with your school code, your role, and the approximate time the problem occurred.
