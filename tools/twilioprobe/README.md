# Twilio delivery evidence probe

`twilioprobe` produces the exact privacy-minimized operational artifact required
for AURA-18.10/18.11. It sends one SMS and one WhatsApp message through the
deployed Gateway, proves Twilio acceptance, waits for the signed status callback,
and reads the final persisted state back from AuraEDU.

The staging recipient must already have enabled `sms` and `whatsapp`
notification subscriptions. Supply runtime values without committing them:

```sh
export AURA_TWILIO_BASE_URL=https://<render-api-gateway-host>
export AURA_TWILIO_RUN_ID=release-2026-07-21-twilio
export AURA_TWILIO_GIT_SHA=<release-sha>
export AURA_TWILIO_TENANT=<staging-tenant>
export AURA_TWILIO_TOKEN=<notifications-read-manage-send-bearer>
export AURA_TWILIO_RECIPIENT_ID=<subscribed-user-uuid>
export AURA_TWILIO_SMS_NUMBER=<approved-e164-recipient>
export AURA_TWILIO_WHATSAPP_NUMBER=<approved-e164-recipient>

cd tools/twilioprobe
GOWORK=off go run . \
  -config ../../release/scenarios/staging-twilio-providers.json \
  -out ../../release/evidence/records/AURA-18.10-18.11/twilio.json
```

The artifact contains only release provenance, timestamps, pass/fail checks and
16-character fingerprints. It never retains tenant/user identifiers, bearer
tokens, phone numbers, provider SIDs, message bodies, callback bodies or provider
responses. Output is created with mode `0600` and cannot overwrite an existing
record.
