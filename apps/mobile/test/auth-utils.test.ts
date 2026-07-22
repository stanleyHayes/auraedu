import assert from "node:assert/strict";
import test from "node:test";

import {
  isMobileRole,
  normalizeGatewayApiUrl,
  normalizeTenantBranding,
  normalizeTokenPair,
  parseStoredSession,
} from "../src/auth-utils.ts";

void test("mobile accepts learner-facing roles only", () => {
  assert.equal(isMobileRole("teacher"), true);
  assert.equal(isMobileRole("parent"), true);
  assert.equal(isMobileRole("student"), true);
  assert.equal(isMobileRole("school_admin"), false);
  assert.equal(isMobileRole("platform_super_admin"), false);
});

void test("gateway URL normalization removes whitespace and trailing slashes", () => {
  assert.equal(normalizeGatewayApiUrl(" https://api.auraedu.com/// "), "https://api.auraedu.com");
  assert.throws(() => normalizeGatewayApiUrl("http://api.auraedu.com"), /HTTPS/);
  assert.equal(normalizeGatewayApiUrl("http://127.0.0.1:8080/", false), "http://127.0.0.1:8080");
  assert.throws(() => normalizeGatewayApiUrl("javascript:alert(1)"), /HTTP/);
  assert.throws(() => normalizeGatewayApiUrl("https://user:secret@api.auraedu.com"), /credentials/);
});

const tokenPair = {
  access_token: "a".repeat(40),
  refresh_token: "r".repeat(48),
  expires_at: "2026-07-20T18:00:00Z",
  user: {
    id: "user-1",
    name: "Teacher One",
    email: "teacher@example.com",
    role: "teacher",
    tenant_id: "upshs",
    permissions: ["attendance.mark"],
  },
};

void test("token pairs and restored sessions are schema validated", () => {
  const session = normalizeTokenPair(tokenPair);
  assert.equal(session.user.role, "teacher");
  assert.deepEqual(parseStoredSession(JSON.stringify(session)), session);
  assert.throws(() => parseStoredSession("not-json"), /not valid JSON/);
  assert.throws(() => normalizeTokenPair({ ...tokenPair, expires_at: "tomorrow" }), /invalid/);
  assert.throws(
    () => normalizeTokenPair({ ...tokenPair, user: { ...tokenPair.user, role: "school_admin" } }),
    /not valid for AuraEDU Mobile/,
  );
});

void test("tenant branding validates ownership, status, and colors", () => {
  assert.deepEqual(
    normalizeTenantBranding("upshs", {
      tenant_code: "upshs",
      name: "University Practice SHS",
      short: "  UPSHS ",
      status: "active",
      branding: { brand: { primary: "#1557FF", secondary: "#63D5DA" } },
    }),
    {
      code: "upshs",
      name: "University Practice SHS",
      short: "UPSHS",
      status: "active",
      logoUrl: undefined,
      primary: "#1557FF",
      secondary: "#63D5DA",
    },
  );
  assert.throws(
    () =>
      normalizeTenantBranding("upshs", {
        tenant_code: "another-school",
        name: "Other",
        status: "active",
      }),
    /did not match/,
  );
  assert.throws(
    () =>
      normalizeTenantBranding("upshs", {
        tenant_code: "upshs",
        name: "UPSHS",
        status: "suspended",
      }),
    /not active/,
  );
});

void test("invalid tenant colors fall back without trusting server input", () => {
  const branding = normalizeTenantBranding("upshs", {
    tenant_code: "upshs",
    name: "UPSHS",
    status: "active",
    branding: { brand: { primary: "red", secondary: "javascript:alert(1)" } },
  });
  assert.equal(branding.primary, "#1557FF");
  assert.equal(branding.secondary, undefined);
});
