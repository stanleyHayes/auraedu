import assert from "node:assert/strict";
import test from "node:test";

import {
  consumerEventCandidates,
  directProducerEventTypes,
  producerCoverageFailures,
  pythonProducerCoverageFailures,
  runtimeEventFailures,
} from "./validate-events.js";

void test("extracts direct CloudEvent producer types", () => {
  assert.deepEqual(
    directProducerEventTypes(`tenancy.NewCloudEvent("lead.created.v1", source, id, tenant, data)`),
    ["lead.created.v1"],
  );
});

void test("extracts static Go consumer candidates", () => {
  const source = `
    events := []string{"lead.created", "attendance.marked.v1"}
    eventbus.Subscribe(js, stream, durable, eventType, handler, nil)
  `;
  assert.deepEqual(consumerEventCandidates(source), ["lead.created", "attendance.marked.v1"]);
});

void test("extracts Python subscribed event candidates only from subscriber modules", () => {
  assert.deepEqual(
    consumerEventCandidates(
      `SUBSCRIBED_EVENTS = {"assessment.score_recorded.v1", "attendance.marked.v1"}`,
    ),
    ["assessment.score_recorded.v1", "attendance.marked.v1"],
  );
  assert.deepEqual(consumerEventCandidates(`value = "attendance.marked"`), []);
});

void test("rejects unversioned producers and consumers when a versioned contract exists", () => {
  const contracts = new Set(["lead.created.v1"]);
  assert.deepEqual(
    runtimeEventFailures(
      `
        tenancy.NewCloudEvent("lead.created", source, id, tenant, data)
        events := []string{"lead.created"}
        eventbus.Subscribe(js, stream, durable, eventType, handler, nil)
      `,
      "apps/example/worker.go",
      contracts,
    ),
    [
      "apps/example/worker.go: producer emits unversioned lead.created; use contract type lead.created.v1",
      "apps/example/worker.go: consumer subscribes to unversioned lead.created; use contract type lead.created.v1",
    ],
  );
});

void test("accepts exact versioned producer and consumer contracts", () => {
  const contracts = new Set(["lead.created.v1"]);
  assert.deepEqual(
    runtimeEventFailures(
      `
        tenancy.NewCloudEvent("lead.created.v1", source, id, tenant, data)
        events := []string{"lead.created.v1"}
        eventbus.Subscribe(js, stream, durable, eventType, handler, nil)
      `,
      "apps/example/worker.go",
      contracts,
    ),
    [],
  );
});

void test("rejects producers that strip a version before constructing the envelope", () => {
  assert.deepEqual(
    runtimeEventFailures(
      `tenancy.NewCloudEvent(strings.TrimSuffix(eventType, ".v1"), source, id, tenant, data)`,
      "apps/example/publisher.go",
      new Set(["lead.created.v1"]),
    ),
    [
      "apps/example/publisher.go: producer strips the contract version before constructing a CloudEvent",
    ],
  );
});

void test("requires every production Go CloudEvent service to have a shared schema assertion", () => {
  assert.deepEqual(
    producerCoverageFailures(
      new Set(["student-service", "fees-service"]),
      new Set(["student-service"]),
    ),
    [
      "apps/fees-service: production CloudEvent constructor has no schema-backed AssertEventContract test",
    ],
  );
  assert.deepEqual(
    producerCoverageFailures(new Set(["student-service"]), new Set(["student-service"])),
    [],
  );
});

void test("requires every production Python event service to have a shared schema assertion", () => {
  assert.deepEqual(
    pythonProducerCoverageFailures(
      new Set(["ai-prediction-service", "career-guidance-service"]),
      new Set(["ai-prediction-service"]),
    ),
    [
      "apps/career-guidance-service: production Python event encoder has no schema-backed assert_event_contract test",
    ],
  );
  assert.deepEqual(
    pythonProducerCoverageFailures(
      new Set(["ai-recommendation-service"]),
      new Set(["ai-recommendation-service"]),
    ),
    [],
  );
});
