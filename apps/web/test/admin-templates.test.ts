import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/templates/page.tsx");
const page = readFileSync(pagePath, "utf8");
const actions = readFileSync(join(root, "lib/admin-template-actions.ts"), "utf8");
const form = readFileSync(join(root, "components/admin-template-form.tsx"), "utf8");
const sheet = readFileSync(join(root, "components/admin-template-sheet.tsx"), "utf8");
const rowActions = readFileSync(join(root, "components/admin-template-row-actions.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");
const journeys = readFileSync(join(root, "app/(admin)/admin/journeys/page.tsx"), "utf8");

void test("admin templates route exists with a loading state and navigation entry", () => {
  assert.ok(existsSync(pagePath), "templates page is missing");
  assert.ok(existsSync(join(root, "app/(admin)/admin/templates/loading.tsx")));
  assert.match(navigation, /href: "\/admin\/templates"/);
});

void test("templates page lists templates with channel and status badges and honest states", () => {
  assert.match(page, /\/api\/v1\/notification-templates/);
  assert.match(page, /params\.set\("channel"/);
  assert.match(page, /params\.set\("status"/);
  for (const column of ["Name", "Channel", "Status", "Updated", "Actions"]) {
    assert.match(page, new RegExp(`header: "${column}"`), `column ${column} is missing`);
  }
  assert.match(page, /Could not load templates/);
  assert.match(page, /No templates yet/);
});

void test("template actions hit the notification template CRUD endpoints", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/notification-templates"/);
  assert.match(
    actions,
    /client\.patch\(\s*`\/api\/v1\/notification-templates\/\$\{encodeURIComponent\(templateId\)\}`/,
  );
  assert.match(
    actions,
    /client\.del\(`\/api\/v1\/notification-templates\/\$\{encodeURIComponent\(templateId\)\}`\)/,
  );
  assert.match(actions, /status,\s*\}\)/);
  assert.match(actions, /revalidatePath\("\/admin\/templates"\)/);
  // Templates unblock the journeys builder, which reads the same catalogue.
  assert.match(actions, /revalidatePath\("\/admin\/journeys"\)/);
  assert.match(journeys, /notification-templates\?status=active/);
});

void test("template form covers name, channel, subject and body with validation", () => {
  for (const field of ["name", "channel", "subject_template", "body_template"]) {
    assert.match(form, new RegExp(`name="${field}"`), `field ${field} is missing`);
  }
  assert.match(actions, /Email templates need a subject line/);
  assert.match(form, /\{\{first_name\}\}/);
  assert.match(sheet, /AdminTemplateForm/);
});

void test("template row actions archive, reactivate and confirm deletion", () => {
  assert.match(rowActions, /setTemplateStatusAction/);
  assert.match(rowActions, /"archived" : "active"/);
  assert.match(rowActions, /deleteTemplateAction/);
  assert.match(rowActions, /window\.confirm/);
  assert.match(rowActions, /role="alert"/);
});
