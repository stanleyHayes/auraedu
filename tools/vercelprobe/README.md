# AuraEDU Vercel production evidence probe

`vercelprobe` generates the strict operational JSON profile required by release story `AURA-9.8`. It verifies two independently linked Next.js projects, production-scoped environment keys, a `READY` production deployment of the release Git SHA for each project, public application identity and security headers, and exact CORS preflights from both frontend origins to the deployed API Gateway.

The probe reads only metadata from Vercel. It never requests decrypted environment values and stores only 16-character fingerprints of sanitized observations. The access token, team/project identifiers, environment values, response bodies, and Vercel API payloads are not written to evidence.

## Runtime values

Export these only in the trusted release environment:

- `AURA_VERCEL_RUN_ID`
- `AURA_VERCEL_GIT_SHA`
- `AURA_VERCEL_TOKEN`
- `AURA_VERCEL_TEAM_ID` (optional for a personal Vercel account)
- `AURA_VERCEL_WEB_PROJECT`
- `AURA_VERCEL_MARKETING_PROJECT`
- `AURA_VERCEL_WEB_URL`
- `AURA_VERCEL_MARKETING_URL`
- `AURA_VERCEL_GATEWAY_URL`

The two project values and public origins must be distinct. The portal URL may be `https://auraedugh.vercel.app`; use the marketing project's generated `*.vercel.app` hostname until a custom domain exists.

## Validate and run

```sh
GOWORK=off go run . \
  -config ../../release/scenarios/production-vercel-frontends.json \
  -validate-only

GOWORK=off go run . \
  -config ../../release/scenarios/production-vercel-frontends.json \
  -out ../../release/evidence/records/AURA-9.8/vercel-frontends.json
```

The output path uses exclusive creation and mode `0600`. A failed proof still writes a sanitized diagnostic record, exits non-zero, and cannot satisfy the release validator.

The metadata calls follow Vercel's documented [project lookup](https://vercel.com/docs/rest-api/projects/find-a-project-by-id-or-name), [project environment](https://vercel.com/docs/rest-api/projects/retrieve-the-environment-variables-of-a-project-by-id-or-name), and [deployment listing](https://vercel.com/docs/rest-api/deployments/list-deployments) endpoints.
