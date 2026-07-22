import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

void test("Vercel relays bounded provider callbacks without holding signing secrets", () => {
  const resend = source("../app/api/v1/webhooks/resend/route.ts");
  const twilio = source("../app/api/v1/webhooks/twilio/route.ts");
  const relay = source("../lib/provider-webhook-relay.ts");
  for (const route of [resend, twilio, relay]) {
    assert.doesNotMatch(route, /AUTH_TOKEN|WEBHOOK_SECRET|API_KEY/);
  }
  assert.match(relay, /128 \* 1024/);
  assert.match(relay, /request\.body\.getReader\(\)/);
  assert.match(relay, /total > maxCallbackBytes/);
  assert.doesNotMatch(relay, /request\.arrayBuffer\(\)/);
  assert.match(relay, /AURAEDU_API_URL \?\? process\.env\.NEXT_PUBLIC_API_URL/);
  assert.match(relay, /cache: "no-store"/);
  assert.match(relay, /AbortSignal\.timeout\(10_000\)/);
  assert.match(resend, /"svix-signature"/);
  assert.match(twilio, /"x-twilio-signature"/);
});
