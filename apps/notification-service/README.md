# notification-service

Email, SMS, WhatsApp, in-app notifications (EP-18, L2).

## Run

```bash
cd apps/notification-service
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/server
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/notification-service worker
curl localhost:8080/health
```

## Structure

- `internal/domain` — Message, Template, Subscription, Announcement aggregates.
- `internal/ports` — repository, event publisher, notifier ports.
- `internal/application` — CRUD + send use cases with tenant scope, RBAC and feature flags; `events.go` maps consumed domain events to notifications.
- `internal/adapters` — Postgres, HTTP, eventbus, notifiers.
- `cmd/server` — HTTP entrypoint.
- `cmd/worker` — background event consumer, scheduled-delivery runner and delivery-outbox dispatcher.
- `migrations/0001_init.sql` — Postgres schema + RLS.
- `migrations/0002_announcements.sql` — announcements + worker idempotency ledger (`notification_processed_events`), both with RLS.

## Worker: event → notification side effects

The worker subscribes to notification-worthy domain events and turns each into a message
(`pending` → delivered immediately through the channel notifier):

| Event | Flag | Preferred channel | Notes |
|---|---|---|---|
| `payment.received.v1` | `email_notifications` | email | Payload has no recipient field → tenant in-app inbox. |
| `invoice.created.v1` | `email_notifications` | email | Recipient `student_id`. |
| `attendance.marked.v1` | `sms_notifications` | sms | Only `absent`/`late` alert; `present`/`excused` are acked without side effects. |
| `assessment.score_recorded.v1` | `email_notifications` | email | Recipient `student_id`. |
| `report.published.v1` | `email_notifications` | email | Recipient `student_id`. |
| `offer.issued.v1` | `notifications`, `email_notifications` | email/in-app | Resolves current CRM consent without PII in the event, sends the offer notice, and schedules a leased reminder 48 hours before expiry. |
| `offer.accepted.v1` | `notifications` | none | Cancels pending reminders for the accepted application. |

Rules (`internal/application/events.go`):

- The recipient is derived from the payload (`guardian_id`, `user_id`,
  `student_id`, first non-empty wins). With no resolvable recipient the message
  goes to the tenant's in-app inbox (`recipient_id = tenant_id`).
- When the recipient has no enabled subscription for the preferred channel, the
  worker falls back to `in_app`.
- Flag-off events are acked and skipped (same gate pattern as the
  website-service worker).
- Idempotency: each event id is claimed in `notification_processed_events`
  before the side effect runs; duplicate redeliveries are acked without
  creating a second message. Failed side effects release the claim and Nak so
  JetStream redelivers.
- Delivery outcomes are durable: message `sent`/`failed` state and its
  `notification.sent.v1`/`notification.failed.v1` event commit atomically to a
  FORCE-RLS outbox. The worker publishes with stable event IDs and retries
  broker failures with capped exponential delay.

## Announcements

`POST /api/v1/announcements` (perm `notifications.manage`) creates a
tenant-wide announcement and publishes it to the tenant in-app inbox through
the standard message machinery. `GET /api/v1/announcements[/{id}]`
(`notifications.read`) lists/reads, `DELETE` (`notifications.manage`) removes.
All routes are gated by the `announcements` feature flag.

## Notification providers

`internal/ports.Notifier` is the delivery seam. Local development defaults to
deterministic mock adapters. Production refuses to start with mocks and requires
`NOTIFICATION_PROVIDER=resend`, a preferred `RESEND_API_KEY` (the legacy
`SMTP_PASSWORD` slot remains an accepted fallback), `RESEND_FROM_EMAIL`, and
`RESEND_WEBHOOK_SECRET`. The HTTP adapter uses the Aura message UUID as its
idempotency key and provider tag, then stores the returned provider receipt.
SMTP remains available as a fallback through `SMTP_HOST`, `SMTP_FROM_EMAIL`,
and the optional username, password, port (default 587), and from-name. SMTP
delivery uses TLS 1.2+ (STARTTLS or implicit TLS on 465) and a stable Message-ID.
`PUBLIC_APP_URL` is also mandatory in production and must be a clean HTTPS
origin. Invite and password-reset mail contain tenant-aware links on that trusted
origin. Each one-time token is URL-encoded into the fragment so it is never sent
to the web server, reverse proxy or access logs; the full link exists only in
the provider envelope and is redacted before the message record is committed.

Configure Resend to POST the `email.sent`, `email.delivered`,
`email.delivery_delayed`, `email.bounced`, `email.complained`, `email.failed`,
and `email.suppressed` events to:

```text
https://<public-api-gateway>/api/v1/webhooks/resend
```

While AuraEDU uses its Vercel hostname before a custom API domain is available,
the deployed Portal relays the same callback at
`https://auraedugh.vercel.app/api/v1/webhooks/resend`. Set the resulting
`whsec_...` signing value as `RESEND_WEBHOOK_SECRET` on the Render
`notification-service`; the worker does not receive that secret.

The service verifies the raw Svix signature before parsing, deduplicates by
`svix-id`, tolerates out-of-order delivery, and suppresses future tenant email
after bounce, complaint, or provider suppression. It stores only a SHA-256
recipient-address hash in delivery feedback. Open/click callbacks are ignored.
`onboarding@resend.dev` is suitable only for Resend's domainless test mode;
replace it with a verified sender when a production domain is available.
Consent-verified admissions mail also receives a signed, 180-day preference
link on `PUBLIC_APP_URL`. The browser removes its token fragment immediately
and posts it to `/api/v1/email-preferences/unsubscribe`; Notification records a
one-way tenant suppression without retaining the address or token. Configure
`NOTIFICATION_UNSUBSCRIBE_SIGNING_KEY` (32+ random characters); Render and
`make local-config` generate it automatically.

SMS and WhatsApp use the same Twilio Programmable Messaging adapter while
remaining independently optional. Configure `TWILIO_ACCOUNT_SID` and
`TWILIO_AUTH_TOKEN`, then either `TWILIO_SMS_FROM` or
`TWILIO_MESSAGING_SERVICE_SID` for SMS and an approved
`TWILIO_WHATSAPP_FROM` for WhatsApp. Recipients must be supplied as E.164
`delivery_address` values. Production accepts only Twilio HTTPS API hosts;
responses are bounded and provider error bodies are never persisted or logged.
Set `TWILIO_STATUS_CALLBACK_URL` to the exact public callback configured on
every outbound message. The current domainless deployment uses:

```text
https://auraedugh.vercel.app/api/v1/webhooks/twilio
```

The Vercel route forwards the untouched form and signature to the Gateway.
Notification verifies the exact URL plus every form field with Twilio's SDK,
correlates the provider SID to AuraEDU's message ID, and projects accepted,
delivered/read, or failed/undelivered/canceled state replay-safely. Only the
normalized recipient's SHA-256 hash is retained; the number, raw callback and
provider error text are not stored or logged.
An omitted channel sender retains the fail-closed unconfigured adapter and can
never report a false successful delivery.

Native clients register their authenticated installation through
`POST /api/v1/device-tokens` and remove it on sign-out. Push messages use the
Expo Push API over HTTPS; `EXPO_ACCESS_TOKEN` enables Expo access-token security.
Provider `DeviceNotRegistered` tickets retire the token so dead installations
are not retried. School events prefer native push when the tenant's
`push_notifications` flag is enabled and the recipient has an active device,
then fall back to the configured channel or in-app inbox.

## Communication journeys

`/api/v1/communication-journeys` is the Growth nurturing boundary for email,
SMS, WhatsApp and in-app follow-up. A journey is created as a draft from active,
matching-channel templates and must be activated by a different authorised
human. Supported CRM/admissions events are an explicit allowlist; conditions
can inspect only non-PII event fields and template interpolation is restricted
to approved fields plus the privately resolved lead first name.

The worker projects each event through a separate durable consumer so a journey
storage retry cannot repeat an existing one-off notification. Enrollment is
transactionally replay-safe and schedules all matching steps. Before every
provider call, the worker rechecks the tenant feature, journey state, current
CRM channel consent, quiet hours and rolling recipient frequency limit.
Cancellation events stop every remaining pending step. The API exposes
enrolled, pending, provider-accepted, delivered, delayed, bounced, complained,
suppressed, failed, skipped and cancelled counts; lifecycle changes
also commit a PII-free `communication.journey_changed.v1` audit event to the
same transactional outbox as the journey mutation.

## Contract

REST: `contracts/openapi/notification.v1.yaml` (managed separately)  
Events: consumes `payment.received`, `invoice.created`, `attendance.marked`,
`assessment.score_recorded`, `report.published`; emits
`contracts/events/notification.sent.v1.json`, `notification.failed.v1.json`.
