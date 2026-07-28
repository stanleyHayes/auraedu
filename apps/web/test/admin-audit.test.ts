import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/audit/page.tsx");
const loadingPath = join(root, "app/(admin)/admin/audit/loading.tsx");
const page = readFileSync(pagePath, "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("admin audit route exists with a loading state and navigation entry", () => {
  assert.ok(existsSync(pagePath), "audit page is missing");
  assert.ok(existsSync(loadingPath), "audit loading state is missing");
  assert.match(navigation, /href: "\/admin\/audit"/);
});

void test("audit page queries the audit logs endpoint with filters and cursor pagination", () => {
  assert.match(page, /\/api\/v1\/audit\/logs/);
  for (const key of ["event_type", "actor_id", "source_service", "from", "to"]) {
    assert.match(page, new RegExp(`"${key}"`), `filter param ${key} is not sent`);
  }
  assert.match(page, /params\.set\("cursor"/);
  assert.match(page, /next_cursor/);
});

void test("audit page renders the required columns and distinguishes failure from zero", () => {
  for (const column of ["occurred_at", "event_type", "source_service", "actor_id", "action"]) {
    assert.match(page, new RegExp(`"${column}"`), `column ${column} is missing`);
  }
  assert.match(page, /Could not load audit logs/);
  assert.match(page, /No audit events yet/);
  assert.match(page, /No events match these filters/);
});

void test("audit page is read-only and tolerates filters the backend ignores", () => {
  assert.doesNotMatch(page, /client\.(post|patch|put|del)\(/);
  assert.match(page, /res\.data \?\? \[\]/);
});
