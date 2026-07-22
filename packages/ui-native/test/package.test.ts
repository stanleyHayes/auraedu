import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const testDirectory = dirname(fileURLToPath(import.meta.url));
const components = readFileSync(join(testDirectory, "../src/components.tsx"), "utf8");
const theme = readFileSync(join(testDirectory, "../src/theme.ts"), "utf8");

void test("native primitives remain app-independent", () => {
  assert.doesNotMatch(components, /apps\/mobile|\.\/auth|useAuth/);
  assert.doesNotMatch(theme, /apps\/mobile|\.\/auth|useAuth/);
});

void test("shared screen motion follows the operating system preference", () => {
  assert.match(components, /AccessibilityInfo\.isReduceMotionEnabled\(\)/);
  assert.match(components, /"reduceMotionChanged"/);
  assert.match(components, /if \(reduceMotion\)/);
});

void test("runtime tenant branding retains a readable foreground", () => {
  assert.match(theme, /brandOverride \?\? colors\.brand/);
  assert.match(theme, /onBrand: readableText\(brand\)/);
});
