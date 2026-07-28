import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/reports/page.tsx");
const actionsPath = join(root, "lib/admin-report-actions.ts");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(actionsPath, "utf8");

void test("admin reports page exists with template management components", () => {
  assert.ok(existsSync(pagePath), "reports page is missing");
  assert.ok(existsSync(actionsPath), "report actions module is missing");
  for (const component of [
    "components/admin-report-template-form.tsx",
    "components/admin-report-template-sheet.tsx",
    "components/admin-report-template-delete-button.tsx",
    "components/admin-report-card-actions.tsx",
  ]) {
    assert.ok(existsSync(join(root, component)), `${component} is missing`);
  }
});

void test("reports page loads cards and templates with honest states", () => {
  assert.match(page, /\/api\/v1\/report-cards\?limit=100/);
  assert.match(page, /\/api\/v1\/report-templates\?limit=100/);
  assert.match(page, /Report cards unavailable/);
  assert.match(page, /Could not load templates/);
  assert.match(page, /No report templates/);
  assert.match(page, /No report cards/);
});

void test("template CRUD follows the report contract", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/report-templates"/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/report-templates\/\$\{encodeURIComponent\(id\)\}`/,
  );
  assert.match(
    actions,
    /client\.del\(`\/api\/v1\/report-templates\/\$\{encodeURIComponent\(id\)\}`/,
  );
  const form = readFileSync(join(root, "components/admin-report-template-form.tsx"), "utf8");
  assert.match(form, /name="name"/);
  assert.match(form, /name="academic_year_id"/);
  assert.match(form, /name="body_template"/);
});

void test("card workflow exposes generate, publish, archive and keeps PDF download", () => {
  assert.match(actions, /\/api\/v1\/report-cards\/\$\{encodeURIComponent\(id\)\}\/generate/);
  // publishing and archiving go through UpdateReportCard status transitions
  assert.match(actions, /setReportCardStatus\(id, "published"/);
  assert.match(actions, /setReportCardStatus\(id, "archived"/);
  assert.match(actions, /revalidatePath\("\/admin\/reports"\)/);
  const cardActions = readFileSync(join(root, "components/admin-report-card-actions.tsx"), "utf8");
  assert.match(cardActions, /\/api\/reports\/\$\{id\}\/download/);
  assert.match(page, /AdminReportCardActions id=\{card\.id\} status=\{card\.status\}/);
});
