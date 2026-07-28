import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const recommendations = readFileSync(
  join(root, "app/(admin)/admin/ai/recommendations/page.tsx"),
  "utf8",
);
const predictions = readFileSync(join(root, "app/(admin)/admin/ai/predictions/page.tsx"), "utf8");
const guidance = readFileSync(join(root, "app/(admin)/admin/ai/guidance/page.tsx"), "utf8");
const list = readFileSync(join(root, "components/admin-ai-oversight.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("AI oversight routes exist with loading states and navigation entries", () => {
  for (const route of ["recommendations", "predictions", "guidance"]) {
    assert.ok(existsSync(join(root, `app/(admin)/admin/ai/${route}/loading.tsx`)), route);
  }
  assert.match(navigation, /href: "\/admin\/ai\/recommendations"/);
  assert.match(navigation, /href: "\/admin\/ai\/predictions"/);
  assert.match(navigation, /href: "\/admin\/ai\/guidance"/);
});

void test("each AI page queries its service list endpoint through the gateway prefix", () => {
  assert.match(recommendations, /\/api\/v1\/ai\/recommendations\/recommendations/);
  assert.match(predictions, /\/api\/v1\/ai\/predictions\/predictions/);
  assert.match(guidance, /\/api\/v1\/ai\/career-guidance\/guidance/);
  for (const page of [recommendations, predictions, guidance]) {
    assert.match(page, /params\.set\("student_id"/);
    assert.match(page, /params\.set\("cursor"/);
    assert.doesNotMatch(page, /client\.(post|patch|put|del)\(/, "AI oversight must be read-only");
  }
});

void test("AI oversight shows unapproved items with status badges, confidence and reasons", () => {
  for (const page of [recommendations, predictions, guidance]) {
    assert.match(page, /"pending"/);
    assert.match(page, /"approved"/);
    assert.match(page, /"rejected"/);
    assert.match(page, /Awaiting review/);
  }
  assert.match(list, /Confidence/);
  assert.match(list, /explanation/);
  assert.match(list, /Could not load/);
  assert.match(list, /If the service requires a student reference/);
});

void test("AI pages distinguish fetch failure from a genuine zero", () => {
  assert.match(recommendations, /No recommendations yet/);
  assert.match(predictions, /No predictions yet/);
  assert.match(guidance, /No guidance yet/);
  assert.match(list, /emptyTitle/);
  assert.match(list, /error \? \(/);
});
