import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

function source(path: string) {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

void test("invite acceptance uses the dedicated public one-time-token boundary", () => {
  const action = source("../app/(auth)/accept-invite/actions.ts");
  assert.match(action, /\/api\/v1\/public\/invites\/\$\{encodeURIComponent\(token\)\}\/accept/);
  assert.match(action, /token\.length < 32/);
  assert.match(action, /passwordLength < 12 \|\| passwordLength > 256/);
  assert.match(action, /auraedu_tenant_code/);
});

void test("invite page reads a server-invisible fragment and removes it from the visible URL", () => {
  const page = source("../app/(auth)/accept-invite/page.tsx");
  assert.match(page, /window\.location\.hash\.slice\(1\)/);
  assert.doesNotMatch(page, /searchParams\.get\("token"\)/);
  assert.match(page, /history\.replaceState/);
  assert.match(page, /role="alert"/);
  assert.match(page, /role="status"/);
  assert.match(page, /autoComplete="new-password"/);
  assert.match(page, /maxLength=\{256\}/);
});

void test("global app authentication bootstraps only from a canonical tenant", () => {
  const proxy = source("../proxy.ts");
  const layout = source("../app/layout.tsx");
  const loginAction = source("../app/(auth)/login/actions.ts");
  assert.match(proxy, /isTenantBootstrapPath/);
  assert.match(proxy, /canonicalTenantCode\(request\.nextUrl\.searchParams\.get\("tenant"\)\)/);
  assert.match(proxy, /isTenantNeutralAppHost\(host\)/);
  assert.match(proxy, /request\.cookies\.get\(TENANT_COOKIE\)/);
  assert.match(layout, /globalAuthEntry/);
  assert.match(loginAction, /submittedTenant \|\| canonicalTenantCode/);
});
