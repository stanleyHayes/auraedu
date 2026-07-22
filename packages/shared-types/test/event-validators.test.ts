import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  validateAssessmentScoreRecordedV1Result,
  validateStaffCreatedResult,
  validateTenantCreatedResult,
  validateTenantDeletedResult,
  validateTenantSettingsUpdatedResult,
  validateTenantUpdatedResult,
} from "../dist/generated/events/validators.js";

void test("generated validators load mixed JSON Schema dialects", () => {
  const draft7Result = validateTenantCreatedResult({
    specversion: "1.0",
    type: "tenant.created.v1",
    source: "tenant-service",
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    time: "2026-07-20T10:00:00Z",
    tenant_id: "school-a",
    data: { tenant_code: "school-a", name: "School A" },
  });
  assert.equal(draft7Result.valid, true, JSON.stringify(draft7Result.errors));
});

void test("assessment score contract example passes its generated validator", async () => {
  const raw = await readFile(
    new URL("../../../contracts/events/assessment.score_recorded.v1.json", import.meta.url),
    "utf8",
  );
  const schema = JSON.parse(raw) as { examples: unknown[] };
  const result = validateAssessmentScoreRecordedV1Result(schema.examples[0]);
  assert.equal(result.valid, true, JSON.stringify(result.errors));
});

void test("assessment score validator rejects missing analytics dimensions", () => {
  const result = validateAssessmentScoreRecordedV1Result({
    specversion: "1.0",
    type: "assessment.score_recorded.v1",
    source: "assessment-service",
    id: "event-1",
    time: "2026-07-19T10:00:00Z",
    tenant_id: "school-a",
    data: {
      score_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
      assessment_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
      student_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
      score: 72,
      max_score: 100,
      recorded_at: "2026-07-19T10:00:00Z",
    },
  });
  assert.equal(result.valid, false);
});

void test("staff producer event uses the versioned contract identity", () => {
  const event = {
    specversion: "1.0",
    type: "staff.created.v1",
    source: "staff-service",
    subject: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    time: "2026-07-19T10:00:00Z",
    tenant_id: "school-a",
    data: {
      staff_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
      staff_type: "teacher",
      name: "Example Teacher",
    },
  };

  const versioned = validateStaffCreatedResult(event);
  assert.equal(versioned.valid, true, JSON.stringify(versioned.errors));
  const staleUnversioned = validateStaffCreatedResult({ ...event, type: "staff.created" });
  assert.equal(staleUnversioned.valid, false);

  const undeclaredEnvelopeField = validateStaffCreatedResult({
    ...event,
    idempotency_key: "must-remain-a-transport-header",
  });
  assert.equal(undeclaredEnvelopeField.valid, false);

  const undeclaredDataField = validateStaffCreatedResult({
    ...event,
    data: { ...event.data, personal_email: "private@example.test" },
  });
  assert.equal(undeclaredDataField.valid, false);
});

void test("tenant lifecycle validators accept the durable outbox payloads and reject PII drift", () => {
  const envelope = {
    specversion: "1.0" as const,
    source: "tenant-service",
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    time: "2026-07-20T10:00:00Z",
    tenant_id: "school-a",
  };

  const updated = validateTenantUpdatedResult({
    ...envelope,
    type: "tenant.updated.v1",
    data: { tenant_code: "school-a", name: "School A", status: "active", plan: "growth" },
  });
  assert.equal(updated.valid, true, JSON.stringify(updated.errors));

  const deleted = validateTenantDeletedResult({
    ...envelope,
    type: "tenant.deleted.v1",
    data: { tenant_code: "school-a" },
  });
  assert.equal(deleted.valid, true, JSON.stringify(deleted.errors));

  const settings = validateTenantSettingsUpdatedResult({
    ...envelope,
    type: "tenant.settings_updated.v1",
    data: { tenant_code: "school-a", locale: "en-GH", timezone: "Africa/Accra" },
  });
  assert.equal(settings.valid, true, JSON.stringify(settings.errors));

  const leakedSettings = validateTenantSettingsUpdatedResult({
    ...envelope,
    type: "tenant.settings_updated.v1",
    data: {
      tenant_code: "school-a",
      locale: "en-GH",
      timezone: "Africa/Accra",
      primary_contact_email: "private@example.test",
    },
  });
  assert.equal(leakedSettings.valid, false);
});
