import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/fees/page.tsx");
const actionsPath = join(root, "lib/admin-fees-actions.ts");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(actionsPath, "utf8");

void test("admin fees page exists with management components", () => {
  assert.ok(existsSync(pagePath), "fees page is missing");
  assert.ok(existsSync(actionsPath), "fees actions module is missing");
  for (const component of [
    "components/admin-fees-structure-form.tsx",
    "components/admin-fees-structure-sheet.tsx",
    "components/admin-fees-delete-structure-button.tsx",
    "components/admin-fees-invoice-form.tsx",
    "components/admin-fees-invoice-sheet.tsx",
  ]) {
    assert.ok(existsSync(join(root, component)), `${component} is missing`);
  }
});

void test("fees page loads structures, invoices and lookup data with honest states", () => {
  assert.match(page, /\/api\/v1\/fee-structures\?limit=100/);
  assert.match(page, /\/api\/v1\/invoices\?limit=100/);
  assert.match(page, /\/api\/v1\/academic-years\?limit=50/);
  assert.match(page, /\/api\/v1\/students\?limit=100/);
  assert.match(page, /Fees unavailable/);
  assert.match(page, /No fee structures/);
  assert.match(page, /No invoices yet/);
  for (const column of ["Structure", "Academic year", "Amount", "Recurrence", "Scope", "Status"]) {
    assert.match(page, new RegExp(`header: "${column}"`), `column ${column}`);
  }
});

void test("fees actions cover structure CRUD against the contract", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/fee-structures"/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/fee-structures\/\$\{encodeURIComponent\(id\)\}`/,
  );
  assert.match(actions, /client\.del\(`\/api\/v1\/fee-structures\/\$\{encodeURIComponent\(id\)\}`/);
  assert.match(actions, /revalidatePath\("\/admin\/fees"\)/);
  // amounts are collected in major units and converted to integer cents
  assert.match(actions, /amount_cents/);
  assert.match(actions, /Math\.round\(Number\(raw\) \* 100\)/);
});

void test("invoice issuing posts one invoice per selected student", () => {
  assert.match(actions, /getAll\("student_ids"\)/);
  assert.match(actions, /client\.post\("\/api\/v1\/invoices", body\)/);
  assert.match(actions, /fee_structure_id: feeStructureId/);
  const form = readFileSync(join(root, "components/admin-fees-invoice-form.tsx"), "utf8");
  assert.match(form, /name="fee_structure_id"/);
  assert.match(form, /name="student_ids"/);
  assert.match(form, /name="due_date"/);
  assert.match(form, /Select shown/);
});
