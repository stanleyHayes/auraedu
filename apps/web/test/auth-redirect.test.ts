import assert from "node:assert/strict";
import test from "node:test";

import { safePostLoginPath } from "../lib/safe-redirect.ts";

void test("applicant login preserves only applicant-owned local paths", () => {
  assert.equal(
    safePostLoginPath("/applicant?programme=verified", "applicant"),
    "/applicant?programme=verified",
  );
  assert.equal(safePostLoginPath("/admin", "applicant"), "/applicant");
  assert.equal(safePostLoginPath("https://evil.example/applicant", "applicant"), "/applicant");
  assert.equal(safePostLoginPath("//evil.example/applicant", "applicant"), "/applicant");
  assert.equal(safePostLoginPath("/\\evil.example", "applicant"), "/applicant");
});

void test("role handoff cannot cross into another portal", () => {
  assert.equal(safePostLoginPath("/teacher/classes", "teacher"), "/teacher/classes");
  assert.equal(safePostLoginPath("/student/results", "teacher"), "/teacher");
  assert.equal(
    safePostLoginPath("/superadmin/tenants", "platform_super_admin"),
    "/superadmin/tenants",
  );
});
