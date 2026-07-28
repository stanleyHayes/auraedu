import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/cbt/page.tsx");
const detailPath = join(root, "app/(admin)/admin/cbt/[id]/page.tsx");
const page = readFileSync(pagePath, "utf8");
const detail = readFileSync(detailPath, "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("admin CBT routes exist with loading states and navigation entry", () => {
  assert.ok(existsSync(pagePath), "CBT list page is missing");
  assert.ok(existsSync(detailPath), "CBT detail page is missing");
  assert.ok(existsSync(join(root, "app/(admin)/admin/cbt/loading.tsx")));
  assert.ok(existsSync(join(root, "app/(admin)/admin/cbt/[id]/loading.tsx")));
  assert.match(navigation, /href: "\/admin\/cbt"/);
});

void test("CBT list page queries exams and submissions for oversight counts", () => {
  assert.match(page, /\/api\/v1\/cbt\/exams\?limit=50/);
  assert.match(page, /\/api\/v1\/cbt\/submissions\?limit=100/);
  assert.match(page, /\/admin\/cbt\/\$\{exam\.id\}/);
  for (const column of ["Exam", "Class / subject", "Status", "Window", "Submissions"]) {
    assert.match(page, new RegExp(`header: "${column.replace("/", "\\/")}"`), `column ${column}`);
  }
  assert.match(page, /Could not load CBT exams/);
  assert.match(page, /No CBT exams yet/);
});

void test("CBT detail page scopes submissions to the exam and stays read-only", () => {
  assert.match(detail, /\/api\/v1\/cbt\/exams\/\$\{encodeURIComponent\(id\)\}/);
  assert.match(detail, /exam_id=\$\{encodeURIComponent\(id\)\}/);
  assert.match(detail, /Could not load the exam/);
  assert.match(detail, /Could not load submissions/);
  assert.match(detail, /No submissions yet/);
  for (const column of ["Student", "Status", "Score", "Submitted at"]) {
    assert.match(detail, new RegExp(`header: "${column}"`), `column ${column}`);
  }
  assert.doesNotMatch(detail, /client\.(post|patch|put|del)\(/);
});
