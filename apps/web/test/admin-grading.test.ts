import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/grading/page.tsx");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(join(root, "lib/admin-grading-actions.ts"), "utf8");
const form = readFileSync(join(root, "components/admin-grading-scale-form.tsx"), "utf8");
const sheet = readFileSync(join(root, "components/admin-grading-scale-sheet.tsx"), "utf8");
const deleteButton = readFileSync(join(root, "components/admin-grading-scale-delete.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("admin grading route exists with a loading state and navigation entry", () => {
  assert.ok(existsSync(pagePath), "grading page is missing");
  assert.ok(existsSync(join(root, "app/(admin)/admin/grading/loading.tsx")));
  assert.match(navigation, /href: "\/admin\/grading"/);
});

void test("grading page lists scales with their grade bands and honest states", () => {
  assert.match(page, /\/api\/v1\/grading-scales/);
  assert.match(page, /range\.min/);
  assert.match(page, /range\.max/);
  assert.match(page, /range\.grade/);
  assert.match(page, /Could not load grading scales/);
  assert.match(page, /No grading scales yet/);
});

void test("grading scale actions hit the academic grading-scale CRUD endpoints", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/grading-scales"/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/grading-scales\/\$\{encodeURIComponent\(scaleId\)\}`/,
  );
  assert.match(
    actions,
    /client\.del\(`\/api\/v1\/grading-scales\/\$\{encodeURIComponent\(scaleId\)\}`\)/,
  );
  assert.match(actions, /revalidatePath\("\/admin\/grading"\)/);
});

void test("grading actions validate bands before posting", () => {
  for (const field of ["band_grade", "band_min", "band_max", "band_remark"]) {
    assert.match(actions, new RegExp(`"${field}"`), `field ${field} is missing`);
  }
  assert.match(actions, /min must not exceed max/);
  assert.match(actions, /Add at least one grade band/);
});

void test("grading form edits bands dynamically and deletion asks for confirmation", () => {
  assert.match(form, /name="band_grade"/);
  assert.match(form, /name="band_min"/);
  assert.match(form, /name="band_max"/);
  assert.match(form, /name="band_remark"/);
  assert.match(form, /Add band/);
  assert.match(sheet, /AdminGradingScaleForm/);
  assert.match(deleteButton, /window\.confirm/);
  assert.match(deleteButton, /loadingLabel="Deleting"/);
});
