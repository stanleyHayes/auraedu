import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const pagePath = join(root, "app/(admin)/admin/delivery/page.tsx");
const detailPath = join(root, "app/(admin)/admin/delivery/[id]/page.tsx");
const page = readFileSync(pagePath, "utf8");
const detail = readFileSync(detailPath, "utf8");
const navigation = readFileSync(join(root, "lib/tenant.ts"), "utf8");

void test("delivery routes exist with loading states and navigation entry", () => {
  assert.ok(existsSync(pagePath), "delivery page is missing");
  assert.ok(existsSync(detailPath), "delivery detail page is missing");
  assert.ok(existsSync(join(root, "app/(admin)/admin/delivery/loading.tsx")));
  assert.ok(existsSync(join(root, "app/(admin)/admin/delivery/[id]/loading.tsx")));
  assert.match(navigation, /href: "\/admin\/delivery"/);
});

void test("delivery page queries messages with channel and status filters plus cursor", () => {
  assert.match(page, /\/api\/v1\/messages\?/);
  assert.match(page, /params\.set\("channel"/);
  assert.match(page, /params\.set\("status"/);
  assert.match(page, /params\.set\("cursor"/);
  for (const channel of ["email", "sms", "whatsapp", "in_app"]) {
    assert.match(page, new RegExp(`"${channel}"`), `channel filter ${channel} is missing`);
  }
  for (const status of ["pending", "sent", "failed", "cancelled"]) {
    assert.match(page, new RegExp(`"${status}"`), `status filter ${status} is missing`);
  }
});

void test("delivery columns make the outcome unmistakable and distinguish failure from zero", () => {
  for (const column of ["Created at", "Channel", "Recipient", "Status", "Template"]) {
    assert.match(page, new RegExp(`header: "${column}"`), `column ${column}`);
  }
  assert.match(page, /deliveryStateStyle/);
  assert.match(page, /deliveryDotStyle/);
  assert.match(page, /Could not load delivery logs/);
  assert.match(page, /No messages yet/);
  assert.match(page, /No messages match these filters/);
  assert.doesNotMatch(page, /client\.(post|patch|put|del)\(/);
});

void test("message detail loads the message and renders status history", () => {
  assert.match(detail, /\/api\/v1\/messages\/\$\{encodeURIComponent\(id\)\}/);
  assert.match(detail, /Status history/);
  assert.match(detail, /status_history/);
  assert.match(detail, /delivery_status/);
  assert.match(detail, /Could not load the message/);
  assert.match(detail, /role="alert"/);
});
