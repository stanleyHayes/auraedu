import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import {
  checkRouteFeature,
  enabledFeatureKeys,
  getRouteFeature,
  isNavigationFeatureVisible,
} from "../lib/features.ts";
import {
  ADMIN_NAV,
  APPLICANT_NAV,
  PARENT_NAV,
  STUDENT_NAV,
  SUPERADMIN_NAV,
  TEACHER_NAV,
} from "../lib/tenant.ts";
import {
  TENANT_NOT_FOUND_HEADER,
  canonicalTenantCode,
  getTenantCodeFromHeaders,
  isTenantNeutralAppHost,
  isTenantNotFound,
  resolveTenantFromHost,
} from "../lib/tenant.ts";

void test("tenant resolution normalizes subdomains and ports", () => {
  assert.equal(resolveTenantFromHost("UPSHS.AURAEDU.COM:443"), "upshs");
  assert.equal(resolveTenantFromHost("aboom-ame-zion-c.localhost:3000"), "aboom-ame-zion-c");
  assert.equal(resolveTenantFromHost("app.auraedu.com"), "");
  assert.equal(resolveTenantFromHost("school.example.org"), "");
  assert.equal(isTenantNeutralAppHost("app.auraedu.com:443"), true);
  assert.equal(isTenantNeutralAppHost("app.attacker.example"), false);
});

void test("apex and localhost hosts carry no tenant unless one is configured", () => {
  delete process.env.NEXT_PUBLIC_DEFAULT_TENANT_CODE;
  assert.equal(resolveTenantFromHost("localhost:3000"), "");
  assert.equal(resolveTenantFromHost("127.0.0.1:3101"), "");
  assert.equal(resolveTenantFromHost("auraedu.com"), "");
  process.env.NEXT_PUBLIC_DEFAULT_TENANT_CODE = "upshs";
  assert.equal(resolveTenantFromHost("localhost:3000"), "upshs");
  delete process.env.NEXT_PUBLIC_DEFAULT_TENANT_CODE;
});

void test("tenant bootstrap codes are canonical single DNS labels", () => {
  assert.equal(canonicalTenantCode(" Release-School "), "release-school");
  assert.equal(canonicalTenantCode("school.example.org"), "");
  assert.equal(canonicalTenantCode("-school"), "");
  assert.equal(canonicalTenantCode("school/../../admin"), "");
});

void test("an explicit tenant header takes precedence and not-found is fail closed", () => {
  const headers = new Headers({
    host: "upshs.auraedu.com",
    "x-tenant-code": "aboom-ame-zion-c",
    [TENANT_NOT_FOUND_HEADER]: "1",
  });
  assert.equal(getTenantCodeFromHeaders(headers), "aboom-ame-zion-c");
  assert.equal(isTenantNotFound(headers), true);
});

void test("route feature matching is segment-aware", () => {
  assert.equal(getRouteFeature("/admin/admissions"), "admissions");
  assert.equal(getRouteFeature("/admin/admissions/application-1"), "admissions");
  assert.equal(getRouteFeature("/admin/admissions-other"), null);
  assert.equal(getRouteFeature("/student/cbt-exams/"), "cbt_exams");
  assert.equal(getRouteFeature("/admin/automation"), "growth_autonomous_actions");
});

void test("school A can expose a module while school B stays hidden and denied", () => {
  const schoolA = [{ feature_key: "cbt_exams", is_enabled: true }];
  const schoolB = [{ feature_key: "cbt_exams", is_enabled: false }];

  assert.equal(isNavigationFeatureVisible("cbt_exams", enabledFeatureKeys(schoolA)), true);
  assert.equal(isNavigationFeatureVisible("cbt_exams", enabledFeatureKeys(schoolB)), false);
  assert.equal(checkRouteFeature("/student/cbt-exams", schoolA).enabled, true);
  assert.equal(checkRouteFeature("/student/cbt-exams", schoolB).enabled, false);
});

void test("feature navigation fails closed when the snapshot is unavailable", () => {
  assert.equal(isNavigationFeatureVisible("cbt_exams", null), false);
  assert.equal(isNavigationFeatureVisible(undefined, null), true);
  assert.equal(isNavigationFeatureVisible("cbt_exams", null, true), true);
});

void test("every role navigation destination has a real app route", () => {
  const appRoot = join(dirname(fileURLToPath(import.meta.url)), "..", "app");
  const routes = new Set<string>();
  const visit = (directory: string) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const fullPath = join(directory, entry.name);
      if (entry.isDirectory()) visit(fullPath);
      if (!entry.isFile() || entry.name !== "page.tsx") continue;
      const segments = relative(appRoot, directory)
        .split(sep)
        .filter((segment) => segment && !(segment.startsWith("(") && segment.endsWith(")")));
      routes.add(`/${segments.join("/")}`.replace(/\/$/, "") || "/");
    }
  };
  visit(appRoot);

  const navigation = [
    ADMIN_NAV,
    APPLICANT_NAV,
    PARENT_NAV,
    STUDENT_NAV,
    SUPERADMIN_NAV,
    TEACHER_NAV,
  ];
  for (const groups of navigation) {
    for (const item of groups.flatMap((group) => group.items)) {
      const pathname = item.href.split("?")[0]!;
      assert.equal(routes.has(pathname), true, `${item.label} points to missing route ${pathname}`);
    }
  }
});

void test("foundation settings remain visible without a feature snapshot", () => {
  const settings = ADMIN_NAV.flatMap((group) => group.items).find(
    (item) => item.href === "/admin/settings",
  );
  assert.ok(settings);
  assert.equal(settings.feature, undefined);
  assert.equal(isNavigationFeatureVisible(settings.feature, null), true);
});

void test("protected routes are denied when their feature is absent", () => {
  assert.deepEqual(checkRouteFeature("/parent/reports", []), {
    enabled: false,
    feature: "report_cards",
  });
  assert.deepEqual(
    checkRouteFeature("/parent/reports", [{ feature_key: "report_cards", is_enabled: true }]),
    {
      enabled: true,
      feature: "report_cards",
    },
  );
  assert.deepEqual(checkRouteFeature("/login", []), { enabled: true, feature: null });
});
