import assert from "node:assert/strict";
import test from "node:test";

import {
  ApiError,
  FeatureDisabledError,
  UnauthorizedError,
  createGatewayClient,
} from "../src/index.ts";

void test("gateway client attaches tenant and bearer identity without duplicating slashes", async () => {
  let captured: { input: string; init?: RequestInit } | undefined;
  const client = createGatewayClient({
    baseUrl: "https://gateway.example/",
    getToken: () => "access-token",
    getTenantCode: () => "upshs",
    fetch: (input, init) => {
      const url = input instanceof Request ? input.url : input instanceof URL ? input.href : input;
      captured = { input: url, init };
      return Promise.resolve(Response.json({ ok: true }));
    },
  });

  await client.post("api/v1/students", { name: "Ama" });

  assert.equal(captured?.input, "https://gateway.example/api/v1/students");
  const headers = new Headers(captured?.init?.headers);
  assert.equal(headers.get("authorization"), "Bearer access-token");
  assert.equal(headers.get("x-tenant-code"), "upshs");
  assert.equal(headers.get("content-type"), "application/json");
  assert.equal(captured?.init?.body, JSON.stringify({ name: "Ama" }));
});

void test("gateway client maps authorization errors to typed failures", async (t) => {
  await t.test("feature disabled", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json(
            { code: "feature_disabled", message: "Not enabled", details: "report_cards" },
            { status: 403 },
          ),
        ),
    });
    await assert.rejects(client.get("/api/v1/reports"), (error: unknown) => {
      assert.ok(error instanceof FeatureDisabledError);
      assert.equal(error.feature, "report_cards");
      return true;
    });
  });

  await t.test("gateway nested error shape", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json(
            {
              error: {
                code: "feature_disabled",
                message: "this feature is not enabled for the tenant",
              },
            },
            { status: 403 },
          ),
        ),
    });
    await assert.rejects(client.get("/api/v1/fees"), (error: unknown) => {
      assert.ok(error instanceof FeatureDisabledError);
      assert.equal(error.feature, "unknown");
      assert.equal(error.message, "this feature is not enabled for the tenant");
      return true;
    });
  });

  await t.test("domain httpx error shape with feature details", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json(
            {
              error: "feature_disabled",
              message: "feature is disabled for this tenant",
              details: { feature: "fees" },
            },
            { status: 403 },
          ),
        ),
    });
    await assert.rejects(client.get("/api/v1/fees"), (error: unknown) => {
      assert.ok(error instanceof FeatureDisabledError);
      assert.equal(error.feature, "fees");
      return true;
    });
  });

  await t.test("gateway nested unauthorized shape", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json(
            { error: { code: "unauthorized", message: "invalid or expired access token" } },
            { status: 401 },
          ),
        ),
    });
    await assert.rejects(client.get("/api/v1/profile"), (error: unknown) => {
      assert.ok(error instanceof UnauthorizedError);
      assert.equal(error.message, "invalid or expired access token");
      return true;
    });
  });

  await t.test("unauthorized", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json({ code: "unauthorized", message: "Sign in" }, { status: 401 }),
        ),
    });
    await assert.rejects(client.get("/api/v1/profile"), UnauthorizedError);
  });

  await t.test("other API error", async () => {
    const client = createGatewayClient({
      baseUrl: "https://gateway.example",
      fetch: () =>
        Promise.resolve(
          Response.json({ code: "conflict", message: "Already exists" }, { status: 409 }),
        ),
    });
    await assert.rejects(client.post("/api/v1/items", {}), ApiError);
  });
});

void test("gateway client accepts an empty success response", async () => {
  const client = createGatewayClient({
    baseUrl: "https://gateway.example",
    fetch: () => Promise.resolve(new Response(null, { status: 204 })),
  });
  assert.equal(await client.del("/api/v1/session"), undefined);
});
