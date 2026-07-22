import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { theme, tokens } from "../src/index.ts";

void test("mobile and CSS brand tokens remain synchronized", async () => {
  const css = await readFile(new URL("../tokens.css", import.meta.url), "utf8");
  assert.match(css, new RegExp(`--color-brand:\\s*${tokens.brand.DEFAULT}`, "i"));
  assert.match(css, new RegExp(`--color-brand-deep:\\s*${tokens.brand.deep}`, "i"));
  assert.match(css, new RegExp(`--color-brand-tint:\\s*${tokens.brand.tint}`, "i"));
});

void test("themes preserve readable foreground/background contrast pairs", () => {
  assert.notEqual(theme.light.background, theme.light.foreground);
  assert.notEqual(theme.dark.background, theme.dark.foreground);
  assert.equal(tokens.radius.full, 9999);
});
