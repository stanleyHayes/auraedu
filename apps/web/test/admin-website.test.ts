import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const listPath = join(root, "app/(admin)/admin/website/page.tsx");
const editorPath = join(root, "app/(admin)/admin/website/[pageId]/page.tsx");
const actionsPath = join(root, "lib/admin-website-actions.ts");
const list = readFileSync(listPath, "utf8");
const editor = readFileSync(editorPath, "utf8");
const actions = readFileSync(actionsPath, "utf8");

void test("website CMS routes and components exist", () => {
  assert.ok(existsSync(listPath), "website list page is missing");
  assert.ok(existsSync(editorPath), "section editor page is missing");
  assert.ok(existsSync(actionsPath), "website actions module is missing");
  for (const component of [
    "components/admin-website-page-form.tsx",
    "components/admin-website-page-sheet.tsx",
    "components/admin-website-page-delete-button.tsx",
    "components/admin-website-section-form.tsx",
    "components/admin-website-section-sheet.tsx",
    "components/admin-website-section-delete-button.tsx",
  ]) {
    assert.ok(existsSync(join(root, component)), `${component} is missing`);
  }
});

void test("page list uses the website service prefix and links the editor and live site", () => {
  assert.match(list, /\/api\/v1\/website\/pages\?limit=100/);
  assert.match(list, /\/admin\/website\/\$\{page\.id\}/);
  assert.match(list, /View live site/);
  assert.match(list, /Preview/);
  assert.match(list, /Website unavailable/);
  assert.match(list, /No website pages/);
});

void test("page and section mutations follow the website contract", () => {
  assert.match(actions, /client\.post\("\/api\/v1\/website\/pages"/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/website\/pages\/\$\{encodeURIComponent\(id\)\}`/,
  );
  assert.match(actions, /client\.del\(`\/api\/v1\/website\/pages\/\$\{encodeURIComponent\(id\)\}`/);
  assert.match(actions, /\/api\/v1\/website\/pages\/\$\{encodeURIComponent\(pageId\)\}\/sections/);
  assert.match(
    actions,
    /client\.patch\(`\/api\/v1\/website\/sections\/\$\{encodeURIComponent\(sectionId\)\}`/,
  );
  assert.match(
    actions,
    /client\.del\(`\/api\/v1\/website\/sections\/\$\{encodeURIComponent\(sectionId\)\}`/,
  );
});

void test("section editor renders ordered sections with reorder, edit and delete", () => {
  assert.match(editor, /sort\(\(a, b\) => a\.sort_order - b\.sort_order\)/);
  assert.match(editor, /await moveSectionAction\(page\.id, section\.id, "up"\)/);
  assert.match(editor, /await moveSectionAction\(page\.id, section\.id, "down"\)/);
  assert.match(editor, /Preview live page/);
  assert.match(editor, /No sections yet/);
  assert.match(editor, /Could not load sections/);
});

void test("section form edits the content keys the public renderer reads", () => {
  const form = readFileSync(join(root, "components/admin-website-section-form.tsx"), "utf8");
  for (const name of [
    'name="headline"',
    'name="title"',
    'name="body"',
    'name="cta_label"',
    'name="cta_url"',
    'name="item_title"',
    'name="item_description"',
    'name="item_icon"',
    'name="email"',
    'name="phone"',
    'name="address"',
  ]) {
    assert.ok(form.includes(name), `section form is missing ${name}`);
  }
  // the action assembles content per section type
  assert.match(actions, /buildSectionContent/);
  assert.match(actions, /content\.items = items/);
});
