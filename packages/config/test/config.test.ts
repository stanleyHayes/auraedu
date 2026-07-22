import assert from "node:assert/strict";
import test from "node:test";

import { parseMarketingClientEnv, parseWebClientEnv, parseWebServerEnv } from "../src/index.ts";

void test("web configuration has secure, usable local defaults", () => {
  assert.deepEqual(parseWebClientEnv({}), {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    NEXT_PUBLIC_APP_URL: "http://localhost:3000",
    NEXT_PUBLIC_TENANT_HEADER: "x-tenant-code",
    NEXT_PUBLIC_LOG_LEVEL: "info",
  });
  assert.deepEqual(parseWebServerEnv({}), {});
});

void test("marketing configuration accepts explicit HTTPS endpoints", () => {
  assert.deepEqual(
    parseMarketingClientEnv({
      NEXT_PUBLIC_API_URL: "https://api.auraedu.com",
      NEXT_PUBLIC_MARKETING_URL: "https://auraedu.com",
      NEXT_PUBLIC_LOG_LEVEL: "warn",
    }),
    {
      NEXT_PUBLIC_API_URL: "https://api.auraedu.com",
      NEXT_PUBLIC_MARKETING_URL: "https://auraedu.com",
      NEXT_PUBLIC_LOG_LEVEL: "warn",
    },
  );
});

void test("configuration rejects malformed URLs and empty tenant headers", () => {
  assert.throws(() => parseWebClientEnv({ NEXT_PUBLIC_API_URL: "not-a-url" }));
  assert.throws(() => parseWebClientEnv({ NEXT_PUBLIC_TENANT_HEADER: "" }));
  assert.throws(() => parseWebServerEnv({ API_GATEWAY_INTERNAL_URL: "gateway" }));
});

void test("production public origins fail closed instead of falling back to localhost", () => {
  assert.throws(
    () => parseWebClientEnv({ ENVIRONMENT: "production" }),
    /non-loopback HTTPS origin/,
  );
  assert.throws(
    () =>
      parseMarketingClientEnv({
        ENVIRONMENT: "production",
        NEXT_PUBLIC_API_URL: "http://api.auraedu.com",
        NEXT_PUBLIC_MARKETING_URL: "https://auraedu.com",
      }),
    /non-loopback HTTPS origin/,
  );
  assert.deepEqual(
    parseWebClientEnv({
      ENVIRONMENT: "production",
      NEXT_PUBLIC_API_URL: "https://api.auraedu.com",
      NEXT_PUBLIC_APP_URL: "https://app.auraedu.com",
    }),
    {
      NEXT_PUBLIC_API_URL: "https://api.auraedu.com",
      NEXT_PUBLIC_APP_URL: "https://app.auraedu.com",
      NEXT_PUBLIC_TENANT_HEADER: "x-tenant-code",
      NEXT_PUBLIC_LOG_LEVEL: "info",
    },
  );
});

void test("public origins reject credential and query leakage in every environment", () => {
  assert.throws(
    () => parseWebClientEnv({ NEXT_PUBLIC_API_URL: "https://user:secret@api.auraedu.com" }),
    /credentials/,
  );
  assert.throws(
    () => parseMarketingClientEnv({ NEXT_PUBLIC_MARKETING_URL: "https://auraedu.com?token=x" }),
    /query parameters/,
  );
});
