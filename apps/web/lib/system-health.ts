export type PlatformHealthStatus = "healthy" | "degraded" | "down";
export type DependencyStatus = "healthy" | "degraded" | "unreachable";

export interface DependencyHealthCheck {
  service: string;
  endpoint: string;
  status: DependencyStatus;
  detail: string;
  latency_ms: number;
}

export interface PlatformHealthReport {
  status: PlatformHealthStatus;
  generated_at: string;
  checks: DependencyHealthCheck[];
}

export interface PlatformHealthSummary {
  healthy: number;
  degraded: number;
  unreachable: number;
  slowestMs: number;
}

export function summarizePlatformHealth(report: PlatformHealthReport): PlatformHealthSummary {
  return report.checks.reduce<PlatformHealthSummary>(
    (summary, check) => ({
      healthy: summary.healthy + (check.status === "healthy" ? 1 : 0),
      degraded: summary.degraded + (check.status === "degraded" ? 1 : 0),
      unreachable: summary.unreachable + (check.status === "unreachable" ? 1 : 0),
      slowestMs: Math.max(summary.slowestMs, check.latency_ms),
    }),
    { healthy: 0, degraded: 0, unreachable: 0, slowestMs: 0 },
  );
}
