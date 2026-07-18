# notification-service

Email, SMS, WhatsApp, in-app notifications (EP-18, L2).

## Run

```bash
cd apps/notification-service
DATABASE_URL=postgres://... NATS_URL=nats://... go run ./cmd/server
curl localhost:8080/health
```

## Structure

- `internal/domain` — Message, Template, Subscription, Announcement aggregates.
- `internal/ports` — repository, event publisher, notifier ports.
- `internal/application` — CRUD + send use cases with tenant scope, RBAC and feature flags; `events.go` maps consumed domain events to notifications.
- `internal/adapters` — Postgres, HTTP, eventbus, notifiers.
- `cmd/server` — HTTP entrypoint.
- `cmd/worker` — background event consumer.
- `migrations/0001_init.sql` — Postgres schema + RLS.
- `migrations/0002_announcements.sql` — announcements + worker idempotency ledger (`notification_processed_events`), both with RLS.

## Worker: event → notification side effects

The worker subscribes to five domain events and turns each into a message
(`pending` → delivered immediately through the channel notifier):

| Event | Flag | Preferred channel | Notes |
|---|---|---|---|
| `payment.received.v1` | `email_notifications` | email | Payload has no recipient field → tenant in-app inbox. |
| `invoice.created.v1` | `email_notifications` | email | Recipient `student_id`. |
| `attendance.marked.v1` | `sms_notifications` | sms | Only `absent`/`late` alert; `present`/`excused` are acked without side effects. |
| `assessment.score_recorded.v1` | `email_notifications` | email | Recipient `student_id`. |
| `report.published.v1` | `email_notifications` | email | Recipient `student_id`. |

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

## Announcements

`POST /api/v1/announcements` (perm `notifications.manage`) creates a
tenant-wide announcement and publishes it to the tenant in-app inbox through
the standard message machinery. `GET /api/v1/announcements[/{id}]`
(`notifications.read`) lists/reads, `DELETE` (`notifications.manage`) removes.
All routes are gated by the `announcements` feature flag.

## Notifier seam (MockNotifier → real providers)

`internal/ports.Notifier` is the delivery seam: one implementation per channel,
registered in a `map[channel]ports.Notifier` and selected in
`Service.deliver`. Today `internal/adapters/notifier` provides only the
deterministic `MockNotifier` (fails when the body contains "fail", succeeds
otherwise) for every channel — no email/SMS/WhatsApp leaves the process.

A real provider (SMTP, Twilio, …) drops in without touching domain or
application code: implement `ports.Notifier` in a new adapter and swap it into
the map built in `cmd/server`/`cmd/worker` (currently `notifier.Registry()`).

## Contract

REST: `contracts/openapi/notification.v1.yaml` (managed separately)  
Events: consumes `payment.received`, `invoice.created`, `attendance.marked`,
`assessment.score_recorded`, `report.published`; emits
`contracts/events/notification.sent.v1.json`, `notification.failed.v1.json`.
