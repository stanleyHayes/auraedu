import assert from "node:assert/strict";
import test from "node:test";

import { summarizePlatformHealth, type PlatformHealthReport } from "../lib/system-health.ts";

void test("platform health summary preserves degraded and unreachable dependencies", () => {
  const report: PlatformHealthReport = {
    status: "degraded",
    generated_at: "2026-07-19T18:00:00Z",
    checks: [
      {
        service: "student-service",
        endpoint: "/ready",
        status: "healthy",
        detail: "ready",
        latency_ms: 12,
      },
      {
        service: "billing-service",
        endpoint: "/ready",
        status: "degraded",
        detail: "Service Unavailable",
        latency_ms: 41,
      },
      {
        service: "notification-service",
        endpoint: "/ready",
        status: "unreachable",
        detail: "timeout",
        latency_ms: 3000,
      },
    ],
  };

  assert.deepEqual(summarizePlatformHealth(report), {
    healthy: 1,
    degraded: 1,
    unreachable: 1,
    slowestMs: 3000,
  });
});

void test("platform health summary handles an empty report without inventing availability", () => {
  assert.deepEqual(
    summarizePlatformHealth({ status: "down", generated_at: "2026-07-19T18:00:00Z", checks: [] }),
    { healthy: 0, degraded: 0, unreachable: 0, slowestMs: 0 },
  );
});
