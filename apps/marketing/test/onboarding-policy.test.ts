import assert from "node:assert/strict";
import test from "node:test";

import { isValidIdempotencyKey, publicOnboardingFailure } from "../app/api/onboarding/policy.ts";

void test("onboarding requires a bounded replay key", () => {
  assert.equal(isValidIdempotencyKey(null), false);
  assert.equal(isValidIdempotencyKey("too-short"), false);
  assert.equal(isValidIdempotencyKey("a".repeat(16)), true);
  assert.equal(isValidIdempotencyKey("a".repeat(128)), true);
  assert.equal(isValidIdempotencyKey("a".repeat(129)), false);
});

void test("the public proxy preserves safe client failures", () => {
  assert.deepEqual(publicOnboardingFailure(409), {
    status: 409,
    body: {
      code: "conflict",
      message: "This request key was already used for different information.",
    },
  });
  assert.equal(publicOnboardingFailure(422).status, 422);
  assert.equal(publicOnboardingFailure(429).body.code, "rate_limited");
});

void test("the public proxy hides unexpected upstream errors", () => {
  const failure = publicOnboardingFailure(500);
  assert.equal(failure.status, 502);
  assert.equal(failure.body.code, "submission_failed");
  assert.doesNotMatch(failure.body.message, /upstream|internal|database/i);
});
