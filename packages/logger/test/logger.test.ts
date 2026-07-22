import assert from "node:assert/strict";
import test from "node:test";

import { createLogger, redact } from "../src/index.ts";

void test("redact removes sensitive keys and nested PII", () => {
  assert.deepEqual(
    redact({
      email: "ama@example.com",
      profile: {
        note: "Call +1 202-555-0187 or guardian@example.org",
        refreshToken: "secret-token",
      },
    }),
    {
      email: "[REDACTED]",
      profile: {
        note: "Call [REDACTED_PHONE] or [REDACTED_EMAIL]",
        refreshToken: "[REDACTED]",
      },
    },
  );
});

void test("logger redacts PII in both messages and metadata", () => {
  const writes: string[] = [];
  const original = console.info;
  console.info = (entry?: unknown) => writes.push(String(entry));
  try {
    createLogger({ service: "test", minLevel: "info" }).info("Invite sent to ama@example.com", {
      authorization: "Bearer secret",
      phone: "+1 202-555-0187",
    });
  } finally {
    console.info = original;
  }

  assert.equal(writes.length, 1);
  const entry = JSON.parse(writes[0] ?? "{}") as Record<string, unknown>;
  assert.equal(entry.message, "Invite sent to [REDACTED_EMAIL]");
  assert.equal(entry.authorization, "[REDACTED]");
  assert.equal(entry.phone, "[REDACTED]");
});

void test("logger respects the configured minimum level", () => {
  let writes = 0;
  const original = console.debug;
  console.debug = () => {
    writes += 1;
  };
  try {
    createLogger({ minLevel: "warn" }).debug("not emitted");
  } finally {
    console.debug = original;
  }
  assert.equal(writes, 0);
});
