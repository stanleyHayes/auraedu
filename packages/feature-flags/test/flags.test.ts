import assert from "node:assert/strict";
import test from "node:test";

import { buildFlagMap, isFeatureEnabled, type FeatureSnapshot } from "../src/core.ts";

const snapshot: FeatureSnapshot = {
  tenantCode: "upshs",
  flags: [
    { feature_key: "attendance", is_enabled: true },
    { feature_key: "report_cards", is_enabled: false, plan_required: "Growth" },
  ],
};

void test("feature evaluation fails closed for missing snapshots and keys", () => {
  assert.equal(isFeatureEnabled(null, "attendance"), false);
  assert.equal(isFeatureEnabled(snapshot, "unknown"), false);
});

void test("feature evaluation respects the tenant snapshot", () => {
  assert.equal(isFeatureEnabled(snapshot, "attendance"), true);
  assert.equal(isFeatureEnabled(snapshot, "report_cards"), false);
  assert.equal(buildFlagMap(snapshot).get("report_cards")?.plan_required, "Growth");
});

void test("the latest value wins if a snapshot contains a repeated key", () => {
  assert.equal(
    isFeatureEnabled(
      {
        tenantCode: "upshs",
        flags: [
          { feature_key: "attendance", is_enabled: false },
          { feature_key: "attendance", is_enabled: true },
        ],
      },
      "attendance",
    ),
    true,
  );
});
