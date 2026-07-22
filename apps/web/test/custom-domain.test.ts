import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const form = readFileSync(new URL("../components/tenant-form.tsx", import.meta.url), "utf8");
const card = readFileSync(new URL("../components/custom-domain-card.tsx", import.meta.url), "utf8");
const actions = readFileSync(new URL("../lib/tenant-actions.ts", import.meta.url), "utf8");

void test("generic tenant mutations cannot submit an unverified custom domain", () => {
  assert.doesNotMatch(form, /name=["']domain["']/);
  assert.doesNotMatch(actions, /formData\.get\(["']domain["']\)/);
});

void test("portal exposes DNS verification separately from provider TLS activation", () => {
  assert.match(card, /TXT value — shown once/);
  assert.match(card, /Check DNS now/);
  assert.match(card, /Provider TLS reference/);
  assert.match(actions, /custom-domain\/verify/);
  assert.match(actions, /custom-domain\/activate/);
  assert.match(actions, /custom-domain\/deactivate/);
});
