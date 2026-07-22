import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const root = join(import.meta.dirname, "..");
const components = readFileSync(join(root, "src", "components.tsx"), "utf8");
const tour = readFileSync(join(root, "src", "mobile-tour.tsx"), "utf8");
const layout = readFileSync(join(root, "app", "(app)", "_layout.tsx"), "utf8");
const profile = readFileSync(join(root, "app", "(app)", "profile.tsx"), "utf8");

void test("every native page intro exposes one visible, spoken guide", () => {
  assert.match(components, /getMobileGuide\(title, copy\)/);
  assert.match(components, /accessibilityLabel=\{`How to use \$\{title\}`\}/);
  assert.match(components, /guide\.steps\.map/);
  assert.match(components, /Speech\.speak/);
  assert.match(components, /language: "en-GB"/);
  assert.match(components, /Speech\.stop/);
});

void test("mobile mounts a tenant-user-role scoped replayable coach tour", () => {
  assert.match(layout, /<MobileTour \/>/);
  assert.match(
    tour,
    /auraedu-tour-complete:\$\{session\.user\.role\}:\$\{session\.user\.tenant_id\}:\$\{session\.user\.id\}/,
  );
  assert.match(tour, /AccessibilityInfo\.isReduceMotionEnabled\(\)/);
  assert.match(tour, /accessibilityViewIsModal/);
  assert.match(tour, /AsyncStorage\.setItem\(key, "1"\)/);
  assert.match(profile, /replayMobileTour/);
  assert.match(profile, /Show me around/);
});
