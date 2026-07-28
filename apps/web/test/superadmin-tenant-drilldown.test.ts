import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

const drilldown = "../app/(superadmin)/superadmin/tenants/[code]";

void test("tenant drill-down layout exposes the full sub-navigation", () => {
  const layout = source(`${drilldown}/layout.tsx`);
  assert.match(layout, /SuperadminTenantTabs/);

  const tabs = source("../components/superadmin-tenant-tabs.tsx");
  for (const slug of ["students", "staff", "finance", "attendance", "audit", "delivery"]) {
    assert.match(tabs, new RegExp(`slug: "${slug}"`), `missing ${slug} tab`);
  }
  assert.match(tabs, /usePathname/, "tabs must highlight the active section");
});

void test("every drill-down subpage pins the picked tenant and keeps portal states", () => {
  const pages = ["students", "staff", "finance", "attendance", "audit", "delivery"];
  for (const slug of pages) {
    const page = source(`${drilldown}/${slug}/page.tsx`);
    assert.match(page, /createServerClientForTenant\(code\)/, `${slug} must pin X-Tenant-Code`);
    assert.match(page, /<PageHeader\b/, `${slug} needs the shared portal header`);
    assert.match(page, /<EmptyState\b/, `${slug} needs empty and failure states`);
    assert.match(page, /SuperadminPagination/, `${slug} needs cursor pagination`);
    assert.match(page, /next_cursor/, `${slug} must read the list cursor`);
  }
});

void test("drill-down subpages call the documented tenant-scoped endpoints", () => {
  assert.match(source(`${drilldown}/students/page.tsx`), /\/api\/v1\/students\?/);
  assert.match(source(`${drilldown}/staff/page.tsx`), /\/api\/v1\/staff\?/);
  const finance = source(`${drilldown}/finance/page.tsx`);
  assert.match(finance, /\/api\/v1\/invoices\?/);
  assert.match(finance, /\/api\/v1\/payments\?/);
  const attendance = source(`${drilldown}/attendance/page.tsx`);
  assert.match(attendance, /\/api\/v1\/attendance\?/);
  assert.match(attendance, /date: today/, "attendance must filter to today");
  assert.match(source(`${drilldown}/audit/page.tsx`), /\/api\/v1\/audit\/logs\?/);
  assert.match(source(`${drilldown}/delivery/page.tsx`), /\/api\/v1\/messages\?/);
});

void test("per-tenant audit feed sends the documented filter params", () => {
  const page = source(`${drilldown}/audit/page.tsx`);
  for (const param of ["event_type", "actor_id", "source_service"]) {
    assert.match(page, new RegExp(`${param}: filters\\.${param}`), `missing ${param} filter`);
  }
  assert.match(page, /from: dateToIsoStart\(filters\.from\)/);
  assert.match(page, /to: dateToIsoEnd\(filters\.to\)/);
  assert.match(page, /type="date"/, "date filters must be date inputs");
});

void test("delivery drill-down sends channel, status and recipient filters", () => {
  const page = source(`${drilldown}/delivery/page.tsx`);
  assert.match(page, /channel: filters\.channel/);
  assert.match(page, /status: filters\.status/);
  assert.match(page, /recipient_id: filters\.recipient_id/);
  assert.match(page, /StatCard/, "delivery keeps an operational summary strip");
});

void test("drill-down helper builds honest cursor pagination", () => {
  const helper = source("../lib/superadmin-drilldown.ts");
  assert.match(helper, /params\.set\("limit", String\(DRILLDOWN_PAGE_SIZE\)\)/);
  assert.match(helper, /params\.set\("cursor", cursor\)/);
  assert.match(helper, /if \(!nextCursor\) return null/, "no next link without a cursor");
  assert.match(helper, /key !== "cursor"/, "pagination must not stack stale cursors");
});

void test("drill-down routes have a designed loading state", () => {
  const loading = source(`${drilldown}/loading.tsx`);
  assert.match(loading, /PortalRouteLoading/);
});
