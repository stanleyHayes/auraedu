import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");

void test("email opt-out keeps its signed credential out of URLs and browser API calls", () => {
  const page = source("../app/(auth)/unsubscribe/page.tsx");
  const action = source("../app/(auth)/unsubscribe/actions.ts");
  assert.match(page, /window\.location\.hash\.slice\(1\)/);
  assert.match(page, /history\.replaceState/);
  assert.doesNotMatch(page, /searchParams\.get\("token"\)/);
  assert.match(action, /token\.length < 32 \|\| token\.length > 1024/);
  assert.match(action, /\/api\/v1\/email-preferences\/unsubscribe/);
  assert.match(page, /Essential account and\s+security messages can still reach you/);
});
