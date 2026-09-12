import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const nav = readFileSync(join(root, "lib/tenant.ts"), "utf8");
const settings = readFileSync(join(root, "components/platform-account-settings.tsx"), "utf8");
const users = readFileSync(join(root, "app/(superadmin)/superadmin/users/page.tsx"), "utf8");
const layout = readFileSync(join(root, "app/layout.tsx"), "utf8");
const login = readFileSync(join(root, "app/(auth)/login/page.tsx"), "utf8");
const acceptInvite = readFileSync(join(root, "app/(auth)/accept-invite/page.tsx"), "utf8");
const resetPassword = readFileSync(join(root, "app/(auth)/reset-password/page.tsx"), "utf8");
const passwordInput = readFileSync(
  join(root, "../../packages/ui/src/components/password-input.tsx"),
  "utf8",
);
const sidebar = readFileSync(
  join(root, "../../packages/ui/src/components/app-sidebar.tsx"),
  "utf8",
);

void test("platform navigation exposes identity administration and account preferences", () => {
  assert.match(nav, /Users & access/);
  assert.match(nav, /Account & preferences/);
  assert.match(users, /Users, roles & permissions/);
});

void test("appearance preferences expose all four requested material systems and boot before paint", () => {
  for (const design of ["aura", "neumorphic", "clay", "liquid-glass"]) {
    assert.match(settings, new RegExp(`id: "${design}"`));
  }
  assert.match(settings, /auraedu-design-system/);
  assert.match(layout, /dataset\.designSystem=d/);
});

void test("sidebar resolves one most-specific active destination", () => {
  assert.match(sidebar, /sort\(\(a, b\) => b\.href\.length - a\.href\.length\)/);
  assert.match(sidebar, /const active = item\.href === activeHref/);
});

void test("every portal password form uses the accessible visibility control", () => {
  for (const source of [login, acceptInvite, resetPassword]) {
    assert.match(source, /PasswordInput/);
    assert.doesNotMatch(source, /type="password"/);
  }
  assert.match(passwordInput, /aria-label=\{visible \? hideLabel : showLabel\}/);
  assert.match(passwordInput, /aria-pressed=\{visible\}/);
  assert.match(passwordInput, /type="button"/);
});
