# AuraEDU production-readiness continuation handoff

**Prepared:** 2026-07-22 (Africa/Accra)
**Repository:** `https://github.com/stanleyHayes/auraedu.git`
**Branch:** `main`
**Verified Vercel deployment-attempt commit:** `4150015a6ba90b457c3cc4ae44d5aff2105d7f51`

## 1. Objective that must remain intact

Continue implementing and verifying AuraEDU across foundation, Web, Marketing, Growth, Mobile,
domain services, infrastructure, security, and end-to-end workflows until the platform is
genuinely production ready. Keep `agent_plan.md` synchronized. Do not redefine success around a
smaller local milestone. Stop only for a genuine blocker that requires user input, provider
authority, credentials, approved spend, or another external state change.

Production readiness is proven only when every item in `release/evidence/manifest.json` is
`verified`, its immutable evidence passes `make release-evidence-validate`, and
`make release-readiness` succeeds against the exact published release commit.

## 2. Current Git state and publication history

- `origin/main` and local `main` are aligned at
  `4150015a6ba90b457c3cc4ae44d5aff2105d7f51` before the Node compatibility follow-up described
  below.
- Thirty previously local commits were pushed on 2026-07-22. GitHub initially rejected that range
  because commit `0a273ed` contained a Paystack unit-test fixture shaped like a Stripe test
  secret. The unpublished range was replayed in an isolated worktree, and only that fixture
  changed to `unit-test-paystack-key`.
- The remaining production-readiness corpus was then reviewed and published as one intentional
  snapshot: 1,674 files and roughly 157,000 inserted lines. The staged diff check, repository
  formatter, static security scan, credential-pattern scan, focused Go regressions, and GitHub
  push protection all passed before publication.
- The static review found and removed two AWS documentation credential fixtures, replaced a raw
  unbounded internal Staff response decoder with `platform/httpx.DecodeJSONResponse`, and added an
  oversized-response regression. It also removed three Twilio-shaped test identifiers and added
  Twilio Account SID detection to the repository scanner.
- Local tool caches (`.raven/`, `.impeccable/`), the accidental root Expo `app.json`, nested locally
  compiled service binaries, `.vercel/` linkage state, and local Vercel environment files are
  ignored. They are not product source and must not be committed.
- `.env`, `apps/mobile/.env.local`, `apps/web/.env.local`, and
  `apps/marketing/.env.local` contain local/provider or Vercel session configuration, are ignored,
  and are mode `0600`. Never print or commit their values.

The broad Foundation + Growth product snapshot is no longer stranded in a dirty worktree. Future
publication should stage only deliberate source/documentation updates and must preserve the same
secret and generated-artifact controls.

## 3. Verified internal baseline

The current working implementation, before this documentation-only handoff change, passed:

- `CI=true PATH=/Users/shayford/.local/bin:/opt/homebrew/bin:/usr/local/bin:$PATH make ci-check`
- all 26 Go lint modules with zero issues;
- all Go unit and PostgreSQL integration suites;
- Ruff and Ruff format;
- mypy with zero issues;
- Pyright with zero errors (92 third-party/unknown-type warnings remain non-fatal);
- 106 Python tests with one intentional skip;
- all 81 TypeScript tests and all 21 TypeScript lint/typecheck tasks;
- repository Prettier and `git diff --check`;
- 33 OpenAPI contracts, 119 versioned events, and public-route conformance for 24 Go services;
- generated TypeScript clients/validators/registries and generated Go stubs;
- both Compose topology validations;
- `make release-evidence-validate` (14 verifier tests; 19 tracked and 19 pending);
- `tools/ci/security-static-scan.sh`, including credential patterns and production-source logging
  controls;
- actionlint for the modified workflow set;
- `make dev-verify`: all 65 Compose services present, zero restarts, every application readiness
  endpoint and platform/observability probe healthy.

The root contract pipeline is now reproducible on clean machines: Spectral `6.16.1` is a pinned
workspace dependency, `make contracts-lint` uses `pnpm exec`, and CI no longer mutates global npm
state. Prettier ignores only local agent caches; source formatting is still enforced.

After any material commit, repeat the focused checks first, then the full canonical gates before
changing evidence status.

## 4. Implemented product surface

The working tree contains the broad Foundation + Growth implementation, including:

- multi-tenant platform auth, RBAC, feature registries, tenant resolution, RLS, event contracts,
  code generation, migrations, readiness, structured observability, and bounded event delivery;
- Gateway, Identity, Tenant, Academic, Admissions, Analytics, Assessment, Attendance, Audit,
  Billing, Campaign, CBT, Content, CRM, Fees, File, Knowledge, Market Intelligence, Notification,
  Payment, Report, Staff, Student, Website, AI Orchestrator, AI Prediction, AI Recommendation, and
  Career Guidance services;
- redesigned Marketing, public school websites, applicant, school-admin, teacher, parent,
  student, super-admin, and mobile surfaces using the shared AuraEDU design system;
- admissions conversion, consent-aware communication journeys, governed content generation,
  Growth CRM/campaign/intelligence/analytics, first-admin onboarding, payments, report cards,
  teacher analytics, and provider callback relays;
- Docker/Compose full-stack runtime, Render backend topology, Vercel frontend topology,
  observability, backup/restore drills, staging probes, performance/isolation tooling, release
  evidence validation, security gates, and CI workflows.

Do not infer completeness from this list. `agent_plan.md` and the strict evidence manifest remain
authoritative.

## 5. Vercel state completed in this run

The user confirmed Vercel CLI access. The authenticated installation is:

```text
CLI: /Users/shayford/.nvm/versions/node/v26.5.0/lib/node_modules/vercel/dist/vc.js
Node: /Users/shayford/.nvm/versions/node/v26.5.0/bin/node
Vercel CLI: 56.4.1
Account: hayfordstanley
Team ID/slug: hayfordstanleys-projects
```

Invoke it without relying on the non-interactive shell `PATH`:

```bash
/Users/shayford/.nvm/versions/node/v26.5.0/bin/node \
  /Users/shayford/.nvm/versions/node/v26.5.0/lib/node_modules/vercel/dist/vc.js <command>
```

Current provider facts:

- Separate `auraedugh` (Portal) and `auraedugh-marketing` (Marketing) Vercel projects now exist.
- Both projects are connected to `stanleyHayes/auraedu` with `main` as the production branch.
- Portal is configured as Next.js with root `apps/web`; Marketing is configured as Next.js with
  root `apps/marketing`.
- Both projects include source outside their root directories so pnpm workspace packages are
  available, and affected-project deployment detection is enabled.
- `ENVIRONMENT=production` and `NEXT_PUBLIC_APP_URL=https://auraedugh.vercel.app` are present in
  the Production target of both projects. Their values were never printed.
- The committed Render Blueprint now allows exact-origin Gateway CORS from both the Portal and
  Marketing Vercel hostnames; wildcard access remains prohibited.
- The Portal and Marketing local directories are linked to those projects. Vercel session files
  are ignored and mode `0600`; project IDs and credentials are intentionally not committed.
- The push of `4150015` triggered one Production deployment in each project. Both cloned the
  correct `main` commit, installed all 14 pnpm workspace projects, detected Next.js `16.2.10`, and
  then stopped only at the fail-closed validator with
  `NEXT_PUBLIC_API_URL is required in production`. Both deployments correctly remain `Error`, not
  `READY`, and the Portal production alias still has no successful deployment.
- Vercel's current managed build image uses Node `24.15.0`, while AuraEDU development and CI remain
  pinned to Node `26.5.0`. The root engine range now explicitly supports maintained Node 24 on
  Vercel as well as the pinned Node 26.5+ toolchain; `.nvmrc`, CI, and Docker pins are unchanged.
  A local production-mode compatibility run under Node `24.12.0` compiled both applications,
  completed TypeScript, and generated all 69 Portal and 24 Marketing routes.
- The production API Gateway URL is still missing. The only local `NEXT_PUBLIC_API_URL` is a
  loopback development URL. Production builds intentionally fail closed on that value.
- Render CLI `2.17.0` is installed at `/opt/homebrew/bin/render`, but it is not authenticated and
  has no active workspace. The Gateway URL cannot be discovered or deployed through Render until
  a human completes `render login` and selects the AuraEDU workspace.
- Because the deployed Gateway origin is missing, do not weaken
  `tools/vercel/validate-environment.mjs` just to get a green deployment.

Required Vercel production environment for both projects:

```text
NEXT_PUBLIC_API_URL=https://<deployed-render-gateway-origin>
NEXT_PUBLIC_APP_URL=https://auraedugh.vercel.app
```

Also configure the server-only Gateway origin where applicable:

```text
AURAEDU_API_URL=https://<deployed-render-gateway-origin>
API_GATEWAY_URL=https://<deployed-render-gateway-origin>
```

Next Vercel steps after the backend Gateway is live:

1. Authenticate Render CLI, select the correct workspace, apply/verify the committed Blueprint,
   and capture the public `api-gateway` HTTPS origin.
2. Add `NEXT_PUBLIC_API_URL` and `AURAEDU_API_URL` to both Vercel Production targets without
   printing secrets; keep both values identical to the public Gateway origin.
3. Apply the updated two-origin `GATEWAY_CORS_ORIGINS` value when deploying the Render Blueprint.
4. Deploy both Vercel projects from the exact published release commit.
5. Confirm both deployments are `READY`, return expected branded content, and expose the required
   security headers.
6. Confirm the Gateway permits exact-origin CORS preflights from both distinct Vercel origins.
7. Populate `release/scenarios/production-vercel-frontends.json` with the non-secret project and
   URL metadata, run `tools/vercelprobe`, and retain immutable AURA-9.8 evidence.

## 6. Resend and provider state

- `RESEND_API_KEY` is present locally and was never printed.
- The local runtime is configured for Resend's HTTPS API and the development sender.
- Without an owned sending domain, development delivery is suitable only for provider-authorized
  test recipients. Broad first-admin delivery requires a verified Resend sending domain.
- `RESEND_WEBHOOK_SECRET` is still missing because the production callback URL does not exist yet.
- Once Web and Gateway are deployed, create the Resend webhook pointing to:
  `https://auraedugh.vercel.app/api/v1/webhooks/resend`.
- Store the resulting `whsec_...` value only in provider/runtime secrets, then run the committed
  staging email probe and approve received-mailbox plus final delivered-status evidence.
- Twilio SMS/WhatsApp, Paystack production, Expo/EAS, app stores, backup storage/monitoring, Render,
  and custom-domain provider evidence still require their own external credentials or approvals.

## 7. Browser/design QA state

- Earlier local browser reviews covered the redesigned Marketing homepage, responsive navigation,
  onboarding/signup, and the public school/admissions assistant at desktop and mobile widths.
- A new protected-portal audit was started for Admissions, communication journeys, governed
  Content, and teacher Analytics.
- The current in-app browser binding returned `Browser is not available: iab`, so no new screenshots
  from this run are valid audit evidence.
- Do not claim the protected visual audit is complete from source inspection alone. Retry the
  in-app browser when available, capture current screenshots, inspect every saved image, and retain
  desktop/mobile evidence for the exact states named in the release manifest.

## 8. The 19 remaining release evidence items

All non-Done high-level ledger rows match the manifest exactly. There are no untracked open rows.

| ID | Remaining proof |
|---|---|
| AURA-54.1 | Authenticated load scenarios against deployed staging with thresholded JSON. |
| AURA-54.2 | Exact-cardinality 100-tenant scale run in disposable staging. |
| AURA-8.1 | Deployed metrics, traces, logs, dashboards, alerts, receiver, and delivery proof. |
| AURA-50.2 | Two-school, two-direction staging isolation matrix. |
| AURA-48.7 | EAS linkage/credentials, signed builds, and both store submissions. |
| AURA-48.8 | Signed preview OTA plus production-channel promotion. |
| AURA-47.3 | Provider-backed onboarding and first-admin invite delivery. |
| AURA-57.2 | Desktop/mobile public assistant consent, uncertainty, locale, and handoff states. |
| AURA-58.1 | Desktop/mobile applicant and staff admissions conversion flows. |
| AURA-58.3 | Journey authoring/activation/outcomes/suppression/unsubscribe plus email gate. |
| AURA-59.2 | Real grounded generation plus policy/review/approval/denied-publish visual proof. |
| AURA-18.9 | Resend accepted, persisted, signed-webhook-correlated, delivered proof. |
| AURA-18.10/18.11 | Approved Twilio SMS and WhatsApp accepted/delivered/persisted proof. |
| AURA-9.3 | Approved spend, resource sizing/scaling, and zero-downtime provider observation. |
| AURA-9.4 | Render PITR, deployed backups, Object Lock, monitors, hosted restore/cutover. |
| AURA-9.5 | Owned domain, certificate, activation, HTTPS/CORS/routing evidence. |
| AURA-9.1 | Deployed Render health/readiness/identity/routes/runtime/non-root proof. |
| AURA-9.8 | Two linked Vercel projects, deployments, public HTTPS, and exact CORS. |
| AURA-21.9 | Authenticated teacher analytics populated/empty/dependency-failure visual proof. |

Do not manually mark these Done. Add bounded, secret-free evidence artifacts and let
`tools/release/verify-readiness.mjs` enforce the semantic proof.

## 9. Immediate continuation order

1. Have a human run `render login`, select the AuraEDU workspace, and confirm the intended Render
   account can create the Blueprint resources and paid Starter Gateway.
2. Apply/verify `render.yaml`, wait for the Gateway and its private dependencies to become ready,
   and capture the public Gateway HTTPS origin.
3. Complete the two missing Vercel Gateway environment keys, apply the committed exact-origin
   CORS configuration, and deploy both already linked projects.
4. Run `tools/vercelprobe` against the exact deployed Git SHA and retain AURA-9.8 evidence.
5. Create the Resend webhook against the Portal relay and run the committed provider probe.
6. Retry protected browser QA and capture the exact manifest states.
7. Execute the remaining provider/staging/store/DR/performance evidence runs.
8. Update `agent_plan.md`, bind evidence to the exact release SHA, run
   `make release-evidence-validate`, and finally run `make release-readiness`.

## 10. Safety and truthfulness rules for the next agent

- Preserve deliberate local edits and ignored provider/session files; never use destructive Git
  cleanup as a shortcut.
- Read `AGENTS.md`, `CLAUDE.md`, `DESIGN_SYSTEM.md`, `agent_plan.md`, and the release manifest before
  editing.
- Respect lane ownership and contracts-first changes. Generated files are regenerated, never
  hand-edited.
- Do not expose provider secrets in logs, chat, evidence, commits, or screenshots.
- Do not weaken production validators, security checks, RLS, RBAC, feature gates, or evidence
  semantics to make a deployment pass.
- Do not call the platform production-ready while any manifest item is pending, either Vercel
  project has no matching `READY` deployment, or the Gateway is not publicly ready.
- When an external proof becomes possible, execute it and retain evidence instead of replacing it
  with local inference.
