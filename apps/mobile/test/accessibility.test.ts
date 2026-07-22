import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const sourceRoots = ["../app", "../src", "../../../packages/ui-native/src"].map((path) =>
  fileURLToPath(new URL(path, import.meta.url)),
);

function sourceFiles(root: string): string[] {
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const path = join(root, entry.name);
    return entry.isDirectory() ? sourceFiles(path) : extname(entry.name) === ".tsx" ? [path] : [];
  });
}

const sources = sourceRoots.flatMap(sourceFiles).map((path) => ({
  path,
  source: readFileSync(path, "utf8"),
}));

function openingTags(source: string, component: string): string[] {
  return [...source.matchAll(new RegExp(`<${component}\\b[\\s\\S]*?>`, "g"))].map(([tag]) => tag);
}

function selfClosingTags(source: string, component: string): string[] {
  return [...source.matchAll(new RegExp(`<${component}\\b[\\s\\S]*?\\/>`, "g"))].map(
    ([tag]) => tag,
  );
}

void test("mobile uses skeleton loading states instead of bare activity spinners", () => {
  for (const { path, source } of sources) {
    assert.doesNotMatch(source, /\bActivityIndicator\b/, path);
  }
});

void test("shared mobile motion respects the operating system reduce-motion setting", () => {
  const components = sources.find(({ path }) =>
    path.includes("/packages/ui-native/src/components.tsx"),
  );
  assert.ok(components, "shared mobile components must be present");
  assert.match(components.source, /AccessibilityInfo\.isReduceMotionEnabled\(\)/);
  assert.match(components.source, /"reduceMotionChanged"/);
  assert.match(components.source, /if \(reduceMotion\)/);
  assert.match(components.source, /Animated\.timing/);
});

void test("interactive text fields and touch targets expose their semantics", () => {
  for (const { path, source } of sources) {
    for (const tag of selfClosingTags(source, "TextInput")) {
      assert.match(tag, /accessibilityLabel=/, `${path}: TextInput needs an accessibilityLabel`);
    }
    for (const tag of openingTags(source, "TouchableOpacity")) {
      assert.match(tag, /accessibilityRole=/, `${path}: touch target needs an accessibilityRole`);
    }
    for (const tag of openingTags(source, "Text")) {
      assert.doesNotMatch(tag, /\bonPress=/, `${path}: use a semantic pressable instead of Text`);
    }
  }
});

void test("every mobile route title is exposed as a heading", () => {
  const routeFiles = sources.filter(
    ({ path }) => path.includes("/app/") && !path.endsWith("_layout.tsx"),
  );
  for (const { path, source } of routeFiles) {
    if (source.includes("style={styles.title}")) {
      assert.match(
        source,
        /<Text\s+accessibilityRole="header"\s+style=\{styles\.title\}>/,
        `${path}: route title needs header semantics`,
      );
    }
  }
});
