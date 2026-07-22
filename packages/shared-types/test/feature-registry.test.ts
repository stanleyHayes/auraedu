import assert from "node:assert/strict";
import test from "node:test";

import { Features } from "../dist/index.js";

void test("generated TypeScript feature registry is complete and unique", () => {
  assert.ok(Features.featureDefinitions.length > 0);
  assert.deepEqual(
    Features.featureKeys,
    Features.featureDefinitions.map((feature) => feature.key),
  );
  assert.equal(new Set(Features.featureKeys).size, Features.featureKeys.length);

  for (const key of [
    "admissions",
    "push_notifications",
    "growth_crm",
    "growth_website_chat",
  ] as const) {
    assert.ok(Features.featureKeys.includes(key));
  }
});
