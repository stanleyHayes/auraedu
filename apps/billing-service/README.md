# billing-service

SaaS plans, tenant subscriptions, trials and platform invoices (EP-22, L2).

Trial creation, subscription plan changes/upgrades and invoice creation commit their
promised CloudEvents atomically through a FORCE-RLS transactional outbox. The deployed
`billing-service worker` consumes tenant onboarding and publishes pending outbox records
to JetStream with stable IDs and bounded retries.

Subscription/invoice updates, payment-state changes and deletes are intentionally
non-event boundaries until versioned integration contracts are introduced for them.
