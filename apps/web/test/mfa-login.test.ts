import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";

const source = (relative: string) =>
  fs.readFileSync(path.join(import.meta.dirname, relative), "utf8");

void test("privileged login completes MFA without placing secrets in the URL", () => {
  const action = source("../app/(auth)/login/actions.ts");
  const page = source("../app/(auth)/login/page.tsx");

  assert.match(action, /mfa_setup_required/);
  assert.match(action, /\/api\/v1\/auth\/mfa\/verify/);
  assert.match(action, /challenge_token: challengeToken/);
  assert.doesNotMatch(action, /router.*challenge|redirect.*challenge/i);
  assert.match(page, /autoComplete="one-time-code"/);
  assert.match(page, /pattern="\[0-9\]\{6\}"/);
  assert.match(page, /support will never ask for this code or setup key/i);
});
