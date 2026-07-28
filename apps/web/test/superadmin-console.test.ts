import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

const superadmin = "../app/(superadmin)/superadmin";

void test("onboarding queue lists requests with status filter and portal states", () => {
  const page = source(`${superadmin}/onboarding/page.tsx`);
  assert.match(page, /\/api\/v1\/super-admin\/onboarding-requests\?/);
  assert.match(page, /status: filters\.status/);
  assert.match(page, /pending_review/, "pending requests must be decidable");
  assert.match(page, /<PageHeader\b/);
  assert.match(page, /<EmptyState\b/);
  assert.match(page, /SuperadminPagination/);
  assert.match(page, /SuperadminOnboardingDecision/);
});

void test("onboarding decisions hit the contract endpoints with required payloads", () => {
  const actions = source("../lib/superadmin-onboarding-actions.ts");
  assert.match(
    actions,
    /\/api\/v1\/super-admin\/onboarding-requests\/\$\{encodeURIComponent\(requestId\)\}\/approve/,
  );
  assert.match(
    actions,
    /\/api\/v1\/super-admin\/onboarding-requests\/\$\{encodeURIComponent\(requestId\)\}\/reject/,
  );
  assert.match(actions, /\{ tenant_code: tenantCode \}/, "approval requires a tenant code");
  assert.match(actions, /\{ reason \}/, "rejection sends the reason");
  assert.match(actions, /reason\.length < 3/, "rejection reason is mandatory");
  assert.match(actions, /revalidatePath\("\/superadmin\/onboarding"\)/);

  const decision = source("../components/superadmin-onboarding-decision.tsx");
  assert.match(decision, /name="tenant_code"/);
  assert.match(decision, /name="reason"/);
  assert.match(decision, /Sheet/, "decisions must be confirmed in a sheet");
});

void test("audit explorer supports all-tenants and per-tenant reads", () => {
  const page = source(`${superadmin}/audit-logs/page.tsx`);
  assert.match(page, /createServerClientForTenant\(selectedTenant\)/);
  assert.match(page, /createServerClient\(\)/, "all-tenants omits the tenant pin");
  assert.match(page, /\/api\/v1\/audit\/logs\?/);

  const picker = source("../components/superadmin-audit-tenant-picker.tsx");
  assert.match(picker, /All tenants/);
  assert.match(picker, /\?tenant=/);
});

void test("audit explorer sends the documented filter params with cursor pagination", () => {
  const page = source(`${superadmin}/audit-logs/page.tsx`);
  for (const param of ["event_type", "actor_id", "source_service"]) {
    assert.match(page, new RegExp(`${param}: filters\\.${param}`), `missing ${param} filter`);
  }
  assert.match(page, /from: dateToIsoStart\(filters\.from\)/);
  assert.match(page, /to: dateToIsoEnd\(filters\.to\)/);
  assert.match(page, /SuperadminPagination/);
  assert.match(page, /next_cursor/);
  assert.match(page, /<EmptyState\b/, "explorer needs empty and failure states");
  assert.match(page, /type="date"/);
});

void test("billing plan administration uses the contract CRUD endpoints", () => {
  const actions = source("../lib/superadmin-billing-actions.ts");
  assert.match(actions, /client\.post<Plan>\("\/api\/v1\/billing\/plans", body\)/);
  assert.match(actions, /client\.patch<Plan>\(`\/api\/v1\/billing\/plans\//);
  assert.match(actions, /client\.del\(`\/api\/v1\/billing\/plans\//);
  assert.match(actions, /revalidatePath\("\/superadmin\/billing-plans"\)/);

  const page = source(`${superadmin}/billing-plans/page.tsx`);
  assert.match(page, /<SuperadminPlanSheet mode="create" \/>/);
  assert.match(page, /mode="edit"/);
  assert.match(page, /SuperadminDeletePlanButton/);

  const sheet = source("../components/superadmin-plan-sheet.tsx");
  for (const field of ["code", "name", "price", "currency", "billing_interval", "features"]) {
    assert.match(sheet, new RegExp(`name="${field}"`), `plan form needs ${field}`);
  }
});

void test("subscription administration supports plan and status changes", () => {
  const actions = source("../lib/superadmin-billing-actions.ts");
  assert.match(
    actions,
    /\/api\/v1\/billing\/subscriptions\/\$\{encodeURIComponent\(subscriptionId\)\}\/change-plan/,
  );
  assert.match(actions, /\{ plan_id: planId \}/);
  assert.match(actions, /client\.patch<Subscription>\(\s*`\/api\/v1\/billing\/subscriptions\//);
  assert.match(actions, /revalidatePath\("\/superadmin\/subscriptions"\)/);

  const page = source(`${superadmin}/subscriptions/page.tsx`);
  assert.match(page, /SuperadminSubscriptionActions/);

  const component = source("../components/superadmin-subscription-actions.tsx");
  assert.match(component, /name="plan_id"/);
  assert.match(component, /name="status"/);
});

void test("dashboard renders a per-school overview with honest counts", () => {
  const page = source(`${superadmin}/page.tsx`);
  assert.match(page, /Schools overview/);
  assert.match(page, /<DataTable\b/);
  assert.match(page, /boundedCount\(studs\.value\.data, studs\.value\.next_cursor\)/);
  assert.match(page, /`\$\{count\}\+`/, "counts hitting the page limit keep the + suffix");
  assert.match(page, /\/api\/v1\/students\?limit=100/);
  assert.match(page, /\/api\/v1\/staff\?limit=100/);
  assert.match(page, /\/api\/v1\/billing\/subscriptions\?limit=100/);
  assert.match(page, /\/superadmin\/tenants\/\$\{s\.code\}/, "rows drill into the tenant console");
});
