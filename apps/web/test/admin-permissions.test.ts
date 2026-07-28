import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const page = readFileSync(join(root, "app/(admin)/admin/users/page.tsx"), "utf8");
const actions = readFileSync(join(root, "lib/admin-permission-actions.ts"), "utf8");
const groups = readFileSync(join(root, "lib/admin-permission-groups.ts"), "utf8");
const editor = readFileSync(join(root, "components/admin-permission-editor.tsx"), "utf8");

void test("users page loads the permission catalogue alongside users and roles", () => {
  assert.match(page, /\/api\/v1\/permissions/);
  assert.match(page, /Promise\.allSettled/);
  assert.match(page, /AdminPermissionEditor/);
  assert.match(page, /The permission catalogue is unavailable/);
});

void test("permission action replaces grants through PATCH /users/{id}", () => {
  assert.match(actions, /"use server"/);
  assert.match(actions, /client\.patch\(`\/api\/v1\/users\/\$\{encodeURIComponent\(userId\)\}`/);
  assert.match(actions, /permissions: permissionKeys\(data\)/);
  assert.match(actions, /revalidatePath\("\/admin\/users"\)/);
});

void test("permission action surfaces 403 grant validation honestly", () => {
  assert.match(actions, /ApiError/);
  assert.match(actions, /error\.status === 403/);
  assert.match(actions, /only grant permissions you hold yourself/);
});

void test("permission keys are grouped by resource for the matrix", () => {
  assert.match(groups, /groupPermissionsByResource/);
  assert.match(groups, /lastIndexOf\("\."\)/);
  assert.match(editor, /groupPermissionsByResource/);
});

void test("editor renders a checkbox matrix with grant validation warning", () => {
  assert.match(editor, /type="checkbox"/);
  assert.match(editor, /name="permission"/);
  assert.match(editor, /user\.permissions/);
  assert.match(editor, /user\.role/);
  assert.match(editor, /validated server-side/);
  assert.match(editor, /only grant permissions you hold yourself/);
  assert.match(editor, /role="alert"/);
});
