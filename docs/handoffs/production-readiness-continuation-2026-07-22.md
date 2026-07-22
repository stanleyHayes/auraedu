# AuraEDU production-readiness continuation handoff

**Prepared:** 2026-07-22 (Africa/Accra)
**Repository:** `https://github.com/stanleyHayes/auraedu.git`
**Branch:** `main`
**Published baseline:** `f64c39a76e45deee3a14d8386a41c7e956025354`

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
  `f64c39a76e45deee3a14d8386a41c7e956025354`.
- Thirty previously local commits were pushed on 2026-07-22.
- GitHub initially rejected the range because commit `0a273ed` contained a Paystack unit-test
  fixture shaped like a Stripe test secret. The unpublished range was replayed in an isolated
  worktree, and only that fixture changed to `unit-test-paystack-key`.
- The resulting 30 clean commit trees were scanned for the blocked value, the Payment unit suite
  passed, and the clean history was pushed successfully.
- Local `main` was realigned with a mixed reset only after proving the index had no staged changes.
  No working-tree file was discarded.
- A large, intentional production-readiness implementation corpus still exists in the working
  tree. Before this handoff document, the inventory was 671 modified/deleted tracked files and
  550 untracked entries. Treat it as valuable work. Do not run `git reset --hard`, `git clean`,
  `git checkout --`, or broad deletion commands.
- Local tool caches (`.raven/`, `.impeccable/`), the accidental root Expo `app.json`, and nested
  locally compiled service binaries are now ignored. They are not product source and must not be
  committed.
- `.env` and `apps/mobile/.env.local` contain local/provider configuration, are ignored, and have
  mode `0600`. Never print or commit their values.

The immediate publication task is to review, secret-scan, commit, and push the remaining
production-readiness working tree without adding caches, binaries, secrets, or local environment
files.

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

## 5. Vercel state discovered in this run

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

- No AuraEDU project appears in either page of the account's current project list.
- `https://auraedugh.vercel.app/` returns Vercel `404 DEPLOYMENT_NOT_FOUND`.
- The intended Portal project name is `auraedugh`, rooted at `apps/web`.
- Marketing must be a separate project, recommended name `auraedugh-marketing`, rooted at
  `apps/marketing`.
- Both app roots contain committed `vercel.json` files with workspace-aware install/build
  commands and Frankfurt region configuration.
- The local configuration has `NEXT_PUBLIC_APP_URL`/`PUBLIC_APP_URL` set to the intended Portal
  origin and has a usable Resend API key.
- The production API Gateway URL is still missing. The only current `NEXT_PUBLIC_API_URL` is a
  loopback development URL. Production builds intentionally fail closed on that value.
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

1. Create/link `auraedugh` from `apps/web` and `auraedugh-marketing` from `apps/marketing`.
2. Set each project's Git root directory to the exact app directory.
3. Add the production environment variables above without printing secrets.
4. Connect both projects to `stanleyHayes/auraedu` and deploy the exact release commit.
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

1. Run a secret and large-artifact review of the remaining working tree.
2. Stage only product source, contracts, migrations, generated artifacts, tests, documentation,
   workflows, deployment configuration, and accepted design evidence. Exclude env files, caches,
   locally compiled binaries, ephemeral logs, and session state.
3. Run `git diff --cached --check` and inspect the staged summary before committing.
4. Commit and push the verified production-readiness snapshot. If GitHub push protection blocks a
   test fixture, replace it with a deliberately non-secret synthetic value and rewrite only the
   unpublished commit; never bypass push protection for convenience.
5. Obtain or deploy the Render API Gateway origin. This is the immediate external dependency for
   both Vercel projects and the Resend callback relay.
6. Create/link/deploy the two Vercel projects and run `tools/vercelprobe`.
7. Retry protected browser QA and capture the exact manifest states.
8. Execute the remaining provider/staging/store/DR/performance evidence runs.
9. Update `agent_plan.md`, bind evidence to the exact release SHA, run
   `make release-evidence-validate`, and finally run `make release-readiness`.

## 10. Safety and truthfulness rules for the next agent

- Preserve the large dirty working tree; it contains the main body of current product work.
- Read `AGENTS.md`, `CLAUDE.md`, `DESIGN_SYSTEM.md`, `agent_plan.md`, and the release manifest before
  editing.
- Respect lane ownership and contracts-first changes. Generated files are regenerated, never
  hand-edited.
- Do not expose provider secrets in logs, chat, evidence, commits, or screenshots.
- Do not weaken production validators, security checks, RLS, RBAC, feature gates, or evidence
  semantics to make a deployment pass.
- Do not call the platform production-ready while any manifest item is pending or the supplied
  Vercel hostname still returns `DEPLOYMENT_NOT_FOUND`.
- When an external proof becomes possible, execute it and retain evidence instead of replacing it
  with local inference.
