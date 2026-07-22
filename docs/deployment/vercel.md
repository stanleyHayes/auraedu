# Vercel frontend deployment

AuraEDU deploys its two Next.js applications as separate Vercel projects. Render owns the API gateway, private services, workers, data stores, and event infrastructure only.

## Portal project

- Project root directory: `apps/web`
- Include source files outside the root directory: enabled
- Production URL: `https://auraedugh.vercel.app`
- Required production variables:
  - `ENVIRONMENT=production`
  - `NEXT_PUBLIC_API_URL=https://<render-api-gateway-host>`
  - `NEXT_PUBLIC_APP_URL=https://auraedugh.vercel.app`
  - `AURAEDU_API_URL=https://<render-api-gateway-host>` (server-only callback relay; recommended)

## Marketing project

- Project root directory: `apps/marketing`
- Include source files outside the root directory: enabled
- Production URL: use the project's generated `https://<marketing-project>.vercel.app` hostname until a custom domain exists
- Required production variables:
  - `ENVIRONMENT=production`
  - `NEXT_PUBLIC_API_URL=https://<render-api-gateway-host>`
  - `NEXT_PUBLIC_APP_URL=https://auraedugh.vercel.app`
  - `AURAEDU_API_URL=https://<render-api-gateway-host>`

The portal and marketing apps require separate Vercel projects because they are separate Next.js applications. `NEXT_PUBLIC_APP_URL` in Marketing is intentionally the Portal sign-in destination, not the Marketing project's own hostname.

Both app-local `vercel.json` files install from the workspace root, build only their owning application, and run a fail-closed production-origin check before `next build`. Do not commit Vercel project IDs, access tokens, or `.vercel/` state.

The Portal exposes bounded server-only relays for Resend and Twilio at
`/api/v1/webhooks/resend` and `/api/v1/webhooks/twilio`. These give providers a
stable HTTPS hostname before AuraEDU owns a custom domain. They hold no provider
credentials, preserve the signed body and headers, and forward only to the
configured Render Gateway. Keep `AURAEDU_API_URL` and `NEXT_PUBLIC_API_URL`
aligned with that Gateway. Configure Render's `TWILIO_STATUS_CALLBACK_URL` as
`https://auraedugh.vercel.app/api/v1/webhooks/twilio`; create the Resend webhook
against the matching Vercel path and place its generated signing secret only in
Render.

## Production proof

The release candidate is not considered deployed merely because a Vercel hostname exists. `tools/vercelprobe` reads project, environment-key and deployment metadata through Vercel's API, requires the production deployment for each project to be `READY` at the exact release Git SHA, exercises both public apps, and verifies API Gateway CORS from both origins.

Populate the `AURA_VERCEL_*` runtime keys documented in `tools/vercelprobe/README.md`, then create the immutable AURA-9.8 evidence record:

```sh
cd tools/vercelprobe
GOWORK=off go run . \
  -config ../../release/scenarios/production-vercel-frontends.json \
  -out ../../release/evidence/records/AURA-9.8/vercel-frontends.json
```

The probe requests environment metadata without decryption and never stores Vercel credentials, environment values or raw API responses.
