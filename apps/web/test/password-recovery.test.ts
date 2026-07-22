import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

void test("login exposes a real password recovery route", () => {
  assert.match(source("../app/(auth)/login/page.tsx"), /\/forgot-password/);
  assert.match(
    source("../app/(auth)/forgot-password/actions.ts"),
    /\/api\/v1\/auth\/forgot-password/,
  );
});

void test("reset credentials stay in a server-invisible fragment", () => {
  const page = source("../app/(auth)/reset-password/page.tsx");
  const action = source("../app/(auth)/reset-password/actions.ts");
  assert.match(page, /window\.location\.hash\.slice\(1\)/);
  assert.doesNotMatch(page, /searchParams\.get\("token"\)/);
  assert.match(action, /token\.length < 32/);
  assert.match(action, /passwordLength < 12 \|\| passwordLength > 256/);
  assert.match(page, /maxLength=\{256\}/);
  assert.match(action, /REFRESH_TOKEN_COOKIE/);
});
