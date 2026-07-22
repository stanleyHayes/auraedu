import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const page = readFileSync(join(root, "app/(admin)/admin/content/page.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");
const features = readFileSync(join(root, "lib/features.ts"), "utf8");

void test("content studio is tenant-feature guarded and reachable from Growth navigation", () => {
  assert.match(navigation, /Content studio/);
  assert.match(navigation, /growth_content_ai/);
  assert.match(features, /prefix: "\/admin\/content", feature: "growth_content_ai"/);
});

void test("content studio exposes governed generation and independent review without publishing", () => {
  assert.match(page, /\/api\/v1\/content\/generate/);
  assert.match(page, /Idempotency-Key/);
  assert.match(page, /submit-for-review/);
  assert.match(page, /"approve", "reject"/);
  assert.match(page, /The submitting user cannot review their own draft/);
  assert.doesNotMatch(page, /\/api\/v1\/content\/.+\/publish/);
  assert.doesNotMatch(page, /action" value="publish"/);
});

void test("content generation requires institutional policy, key messages, and verified facts", () => {
  assert.match(page, /Save a brand policy first/);
  assert.match(page, /name="key_messages"/);
  assert.match(page, /name="facts"/);
  assert.match(page, /Facts are required/);
  assert.match(page, /No publish button by design/);
});
