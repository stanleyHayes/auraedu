import type { GatewayClient } from "@auraedu/api-client";
import type { OpenAPI } from "@auraedu/shared-types";
import { EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import {
  Activity,
  BarChart3,
  BookOpenCheck,
  CalendarCheck,
  CircleAlert,
  GraduationCap,
  TrendingDown,
  TrendingUp,
} from "lucide-react";
import type { ReactNode } from "react";

import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { summarizeTeacherAnalytics, type TeacherMetric } from "@/lib/teacher-analytics";

type Metric = OpenAPI.analytics_v1.components["schemas"]["Metric"];
type MetricList = OpenAPI.analytics_v1.components["schemas"]["MetricList"];

const metricNames = [
  "assessments.avg_percentage",
  "assessments.count",
  "attendance.present",
  "attendance.absent",
  "attendance.late",
  "attendance.excused",
  "students.count",
  "reports.count",
] as const;

function dateOnly(value: Date) {
  return value.toISOString().slice(0, 10);
}

function formatPercentage(value: number | null) {
  return value === null ? "—" : value.toLocaleString("en-GB", { maximumFractionDigits: 1 });
}

async function loadMetric(client: GatewayClient, metricName: string, from: string, to: string) {
  const rows: Metric[] = [];
  const seenCursors = new Set<string>();
  let cursor: string | null = null;

  for (let page = 0; page < 25; page += 1) {
    const params = new URLSearchParams({
      limit: "100",
      metric_name: metricName,
      bucket_date_from: from,
      bucket_date_to: to,
    });
    if (cursor) params.set("cursor", cursor);
    const response = await client.get<MetricList>(`/api/v1/analytics/metrics?${params}`);
    rows.push(...(response.data ?? []));
    cursor = response.next_cursor ?? null;
    if (!cursor) return rows;
    if (seenCursors.has(cursor)) throw new Error("Analytics returned a repeated page cursor");
    seenCursors.add(cursor);
  }
  throw new Error(`Analytics exceeded the safe page limit for ${metricName}`);
}

export default async function TeacherAnalyticsPage() {
  await requireAuth();
  const client = await createServerClient();
  const to = new Date();
  const from = new Date(to);
  from.setUTCDate(from.getUTCDate() - 29);
  const fromDate = dateOnly(from);
  const toDate = dateOnly(to);

  let metrics: Record<string, TeacherMetric[]> = {};
  let error: string | null = null;
  try {
    const series = await Promise.all(
      metricNames.map(
        async (name) => [name, await loadMetric(client, name, fromDate, toDate)] as const,
      ),
    );
    metrics = Object.fromEntries(series);
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Analytics could not be loaded";
  }

  const summary = summarizeTeacherAnalytics(metrics);
  const hasData = Object.values(metrics).some((rows) => rows.length > 0);
  const trendPositive = summary.improvement !== null && summary.improvement >= 0;
  const highestDaily = Math.max(100, ...summary.dailyPerformance.map((point) => point.value));

  return (
    <div className="space-y-7">
      <PageHeader
        icon={<BarChart3 className="size-6" />}
        title="Teaching analytics"
        description="Thirty days of assessment and attendance signals from the records you are authorised to view. Every number comes from a tenant-scoped Analytics projection."
      />

      {error ? (
        <EmptyState
          title="Teaching analytics are unavailable"
          description={error}
          icon={<CircleAlert className="size-8" />}
        />
      ) : !hasData ? (
        <EmptyState
          title="No teaching signals yet"
          description="Scores, attendance, enrolments and published reports will appear here after their domain events are processed. AuraEDU will not invent values while the dataset is empty."
          icon={<Activity className="size-8" />}
        />
      ) : (
        <>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Reveal>
              <StatCard
                label="Average assessment"
                value={formatPercentage(summary.averagePercentage)}
                unit={
                  summary.averagePercentage === null
                    ? "No scored assessments"
                    : "% of available marks"
                }
                tone={
                  summary.averagePercentage !== null && summary.averagePercentage >= 60
                    ? "ok"
                    : "warn"
                }
              />
            </Reveal>
            <Reveal delay={60}>
              <StatCard
                label="Attendance rate"
                value={formatPercentage(summary.attendanceRate)}
                unit={
                  summary.attendanceRate === null ? "No attendance records" : "% marked present"
                }
                tone={
                  summary.attendanceRate !== null && summary.attendanceRate >= 90 ? "ok" : "warn"
                }
              />
            </Reveal>
            <Reveal delay={120}>
              <StatCard
                label="Scores recorded"
                value={summary.scoreRecords.toLocaleString()}
                unit="records in 30 days"
              />
            </Reveal>
            <Reveal delay={180}>
              <StatCard
                label="Reports published"
                value={summary.reportsPublished.toLocaleString()}
                unit="in 30 days"
              />
            </Reveal>
          </section>

          <section className="grid gap-6 xl:grid-cols-[minmax(0,1.6fr)_minmax(18rem,0.7fr)]">
            <Reveal>
              <article className="overflow-hidden rounded-3xl border border-border bg-surface shadow-sm">
                <div className="flex flex-col gap-3 border-b border-border bg-gradient-to-r from-primary/10 via-transparent to-secondary/10 p-6 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <p className="text-xs font-extrabold uppercase tracking-[0.2em] text-primary">
                      Assessment pulse
                    </p>
                    <h2 className="mt-2 font-heading text-2xl font-black">Performance over time</h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      Sample-weighted percentages; raw marks are never compared across assessments
                      with different maximums.
                    </p>
                  </div>
                  <span className="rounded-full border border-border bg-background/80 px-4 py-2 text-xs font-semibold text-muted-foreground">
                    {fromDate} → {toDate}
                  </span>
                </div>
                <div className="p-6">
                  {summary.dailyPerformance.length ? (
                    <div
                      className="flex h-64 items-end gap-2 border-b border-border px-1 pt-8"
                      role="img"
                      aria-label="Daily average assessment percentages"
                    >
                      {summary.dailyPerformance.map((point) => {
                        const height = Math.max(7, (point.value / highestDaily) * 100);
                        return (
                          <div
                            key={point.date}
                            className="group flex h-full min-w-0 flex-1 items-end"
                          >
                            <div
                              className="relative w-full rounded-t-xl bg-gradient-to-t from-primary to-secondary shadow-[0_-10px_30px_hsl(var(--primary)/0.12)] transition duration-300 group-hover:brightness-110"
                              style={{ height: `${height}%` }}
                            >
                              <span className="absolute -top-7 left-1/2 -translate-x-1/2 text-[0.65rem] font-extrabold tabular-nums text-foreground opacity-0 transition group-hover:opacity-100 group-focus-within:opacity-100">
                                {point.value.toLocaleString("en-GB", { maximumFractionDigits: 1 })}%
                              </span>
                              <span className="sr-only">
                                {point.date}: {point.value.toFixed(1)}%
                              </span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="flex min-h-64 items-center justify-center rounded-2xl border border-dashed border-border bg-muted/30 p-8 text-center text-sm text-muted-foreground">
                      Percentage trends begin when score events include a valid maximum score.
                    </div>
                  )}
                </div>
              </article>
            </Reveal>

            <div className="grid content-start gap-4">
              <Reveal delay={80}>
                <InsightCard
                  icon={
                    trendPositive ? (
                      <TrendingUp className="size-5" />
                    ) : (
                      <TrendingDown className="size-5" />
                    )
                  }
                  eyebrow="Period movement"
                  value={
                    summary.improvement === null
                      ? "Not enough data"
                      : `${summary.improvement >= 0 ? "+" : ""}${summary.improvement.toFixed(1)} points`
                  }
                  description="Recent daily average compared with the earlier half of this 30-day window."
                  positive={trendPositive}
                />
              </Reveal>
              <Reveal delay={140}>
                <InsightCard
                  icon={<GraduationCap className="size-5" />}
                  eyebrow="New enrolments"
                  value={summary.newEnrolments.toLocaleString()}
                  description="Student-enrolled events received during this reporting window."
                />
              </Reveal>
              <Reveal delay={200}>
                <div className="rounded-2xl border border-border bg-foreground p-5 text-background">
                  <div className="flex items-center gap-2 text-secondary">
                    <BookOpenCheck className="size-5" />
                    <p className="text-xs font-extrabold uppercase tracking-[0.18em]">
                      Reading the data
                    </p>
                  </div>
                  <p className="mt-3 text-sm leading-6 text-background/75">
                    Use the movement as a prompt for review, not a verdict on a learner or teacher.
                    Check curriculum coverage, assessment difficulty and attendance context before
                    acting.
                  </p>
                </div>
              </Reveal>
            </div>
          </section>

          <Reveal delay={120}>
            <section className="grid gap-4 rounded-2xl border border-border bg-muted/35 p-5 md:grid-cols-3">
              <Signal
                icon={<CalendarCheck className="size-5" />}
                title="Attendance denominator"
                copy="Present, absent, late and excused records are included. Missing marks are not treated as absences."
              />
              <Signal
                icon={<BarChart3 className="size-5" />}
                title="Comparable scores"
                copy="The trend uses score ÷ maximum score, then weights each aggregate by its real sample count."
              />
              <Signal
                icon={<Activity className="size-5" />}
                title="Projection freshness"
                copy="Values update from durable domain events. An empty state is shown when no verified projection exists."
              />
            </section>
          </Reveal>
        </>
      )}
    </div>
  );
}

function InsightCard({
  icon,
  eyebrow,
  value,
  description,
  positive,
}: {
  icon: ReactNode;
  eyebrow: string;
  value: string;
  description: string;
  positive?: boolean;
}) {
  return (
    <article className="rounded-2xl border border-border bg-surface p-5 shadow-sm">
      <div
        className={`flex size-10 items-center justify-center rounded-xl ${positive === false ? "bg-amber-100 text-amber-800" : "bg-primary/10 text-primary"}`}
      >
        {icon}
      </div>
      <p className="mt-4 text-xs font-extrabold uppercase tracking-[0.18em] text-muted-foreground">
        {eyebrow}
      </p>
      <p className="mt-1 font-heading text-2xl font-black">{value}</p>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
    </article>
  );
}

function Signal({ icon, title, copy }: { icon: ReactNode; title: string; copy: string }) {
  return (
    <div className="flex gap-3">
      <div className="mt-0.5 text-primary">{icon}</div>
      <div>
        <h3 className="text-sm font-bold">{title}</h3>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{copy}</p>
      </div>
    </div>
  );
}
