import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const page = readFileSync(join(root, "app/(admin)/admin/journeys/page.tsx"), "utf8");
const builder = readFileSync(join(root, "components/journey-builder.tsx"), "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");
const features = readFileSync(join(root, "lib/features.ts"), "utf8");

void test("communication journeys are feature-guarded and reachable from Growth navigation", () => {
  assert.match(navigation, /Communication journeys/);
  assert.match(navigation, /\/admin\/journeys.*growth_crm/);
  assert.match(features, /prefix: "\/admin\/journeys", feature: "growth_crm"/);
});

void test("journey studio exposes the complete governed automation controls", () => {
  assert.match(page, /\/api\/v1\/communication-journeys/);
  assert.match(page, /Review and activate/);
  assert.match(page, /<Reveal\b/);
  assert.match(builder, /quiet_start/);
  assert.match(builder, /frequency_limit/);
  assert.match(builder, /cancel_on_events/);
  assert.match(builder, /step_condition_operator/);
  assert.match(builder, /Consent and tenant features are checked again at delivery/);
});

void test("journey builder retains bounded steps and approved template selection", () => {
  assert.match(builder, /steps\.length >= 10/);
  assert.match(builder, /No matching template/);
  assert.doesNotMatch(builder, /contentEditable/);
});
