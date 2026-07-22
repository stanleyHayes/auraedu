import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const appDir = fileURLToPath(new URL("../app/(app)", import.meta.url));

void test("every mobile workspace uses the shared animated screen system", () => {
  const pages = readdirSync(appDir)
    .filter((name) => name.endsWith(".tsx") && name !== "_layout.tsx")
    .map((name) => join(appDir, name));

  for (const path of pages) {
    const page = readFileSync(path, "utf8");
    assert.match(page, /<Screen\b/, `${path} must use the shared atmospheric screen`);
    if (!path.endsWith("index.tsx")) {
      assert.match(page, /PageIntro|ModuleCard/, `${path} must use the shared mobile hierarchy`);
    }
  }
});

void test("mobile navigation and sign-in retain the marketing palette", () => {
  const layout = readFileSync(join(appDir, "_layout.tsx"), "utf8");
  const signIn = readFileSync(join(dirname(appDir), "sign-in.tsx"), "utf8");
  const primitives = readFileSync(
    fileURLToPath(new URL("../../../packages/ui-native/src/components.tsx", import.meta.url)),
    "utf8",
  );

  assert.match(layout, /colors\.midnight/);
  assert.match(layout, /colors\.signal/);
  assert.match(layout, /SymbolView/);
  assert.doesNotMatch(layout, /gridCell|profileHead|noticeBell|todayRing/);
  assert.match(signIn, /auraedu-logo-light\.png/);
  assert.match(signIn, /colors\.midnight/);
  assert.match(primitives, /AccessibilityInfo\.isReduceMotionEnabled/);
});
