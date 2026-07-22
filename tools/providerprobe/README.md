# Deployed email-provider proof

This tool proves that the deployed Notification Service handed a staging email envelope to Resend, persisted the accepted result, and later projected Resend's signed delivery webhook. It creates one uniquely labelled email message, invokes the synchronous send route, reads the accepted state back, then polls until the same record reports `delivery_status=delivered`. A successful API call or mailbox observation alone is insufficient.

The staging recipient must already have an enabled email notification subscription. Use a dedicated release mailbox and a release operator with `notifications.send` and `notifications.read`. Supply every sensitive value at runtime:

```sh
export AURA_PROVIDER_BASE_URL=https://staging-api.auraedu.com
export AURA_PROVIDER_RUN_ID=release-2026-07-20-email
export AURA_PROVIDER_GIT_SHA=abcdef1234567890
export AURA_PROVIDER_TENANT=release-school
export AURA_PROVIDER_TOKEN='...'
export AURA_PROVIDER_RECIPIENT_ID=0198f0db-7d3d-7000-8000-000000000001
export AURA_PROVIDER_EMAIL=release-inbox@example.org

GOWORK=off go run . \
  -config ../../release/scenarios/staging-email-provider.json \
  -out ../../release/evidence/records/AURA-18.9/email-provider-2026-07-20.json
```

The output uses exclusive-create mode `0600`. It stores the environment, deployed Git revision, timing, four bounded outcomes, Resend acceptance, persisted `sent`, webhook-projected `delivered`, and SHA-256-derived fingerprints. It never stores the bearer token, tenant code, recipient address, recipient ID, message ID, subject, body, provider response or API response body. The release mailbox should independently retain the received message for human review; the repository artifact proves the API and signed feedback path without retaining message contents.
