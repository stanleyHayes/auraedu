import assert from "node:assert/strict";
import test from "node:test";

import { buildContactOnboardingRequest } from "../app/api/contact/policy.ts";

const valid = {
  name: "Ama Serwaa",
  school: "University Practice Senior High School",
  email: "ama@ups.edu.gh",
  phone: "+233 20 000 0000",
  country: "gh",
  interest: "demo",
  message: "We want to replace spreadsheet fees tracking this term.",
  accepted_terms: true,
  website: "",
};

void test("a valid contact message is composed into the onboarding intake", () => {
  const request = buildContactOnboardingRequest(valid);
  assert.ok(request);
  assert.equal(request.school_name, valid.school);
  assert.equal(request.administrator_name, valid.name);
  assert.equal(request.country_code, "GH");
  assert.equal(request.plan, "starter");
  assert.equal(request.accepted_terms, true);
  assert.match(request.priorities, /^\[contact:demo\] /);
  assert.match(request.priorities, /spreadsheet fees tracking/);
});

void test("the source tag changes with the declared interest", () => {
  const request = buildContactOnboardingRequest({ ...valid, interest: "support" });
  assert.ok(request);
  assert.match(request.priorities, /^\[contact:support\] /);
});

void test("the honeypot must stay empty", () => {
  assert.equal(buildContactOnboardingRequest({ ...valid, website: "http://spam.example" }), null);
});

void test("consent and plausible contact details are required", () => {
  assert.equal(buildContactOnboardingRequest({ ...valid, accepted_terms: false }), null);
  assert.equal(buildContactOnboardingRequest({ ...valid, email: "not-an-email" }), null);
  assert.equal(buildContactOnboardingRequest({ ...valid, interest: "everything" }), null);
  assert.equal(buildContactOnboardingRequest({ ...valid, message: "hi" }), null);
  assert.equal(buildContactOnboardingRequest({ ...valid, country: "Ghana" }), null);
});

void test("empty phone becomes null and over-long messages are rejected", () => {
  const withoutPhone = buildContactOnboardingRequest({ ...valid, phone: "" });
  assert.ok(withoutPhone);
  assert.equal(withoutPhone.phone, null);
  assert.equal(buildContactOnboardingRequest({ ...valid, message: "x".repeat(3000) }), null);
});
