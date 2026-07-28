import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/users/page.tsx");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(join(root, "lib/admin-user-actions.ts"), "utf8");
const inviteSheet = readFileSync(join(root, "components/admin-user-invite-sheet.tsx"), "utf8");
const roleActions = readFileSync(join(root, "components/admin-user-role-actions.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("admin users route exists with a loading state and navigation entry", () => {
  assert.ok(existsSync(pagePath), "users page is missing");
  assert.ok(existsSync(join(root, "app/(admin)/admin/users/loading.tsx")));
  assert.match(navigation, /href: "\/admin\/users"/);
});

void test("users page lists identity users and loads the assignable role catalogue", () => {
  assert.match(page, /\/api\/v1\/users/);
  assert.match(page, /\/api\/v1\/roles/);
  assert.match(page, /Promise\.allSettled/);
  for (const column of ["Name", "Email", "Role", "Status"]) {
    assert.match(page, new RegExp(`header: "${column}"`), `column ${column} is missing`);
  }
  assert.match(page, /Could not load users/);
  assert.match(page, /No identity users yet/);
  assert.match(page, /Assignable roles are unavailable/);
});

void test("user actions hit the identity invite, role and delete endpoints", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/users\/invites"/);
  assert.match(actions, /\/api\/v1\/users\/\$\{encodeURIComponent\(userId\)\}\/roles/);
  assert.match(actions, /client\.del\(`\/api\/v1\/users\/\$\{encodeURIComponent\(userId\)\}`\)/);
  assert.match(actions, /revalidatePath\("\/admin\/users"\)/);
});

void test("invite and role controls surface MFA requirements and confirm deactivation", () => {
  assert.match(inviteSheet, /authenticator app \(TOTP\)/);
  assert.match(inviteSheet, /privileged role/);
  assert.match(roleActions, /window\.confirm/);
  assert.match(roleActions, /role="alert"/);
  assert.match(inviteSheet, /role="alert"/);
});
