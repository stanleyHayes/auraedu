import {
  Activity,
  ArrowDownRight,
  BarChart3,
  CircleAlert,
  Sparkles,
  Target,
  Waypoints,
} from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Report = OpenAPI.analytics_v1.components["schemas"]["GrowthExecutive"];
type Breakdown = OpenAPI.analytics_v1.components["schemas"]["GrowthBreakdown"];
type Programme = OpenAPI.admissions_v1.components["schemas"]["Programme"];
interface ExecutiveAnswer {
  answer: string;
  confidence: "low" | "medium" | "high";
  source_datasets: string[];
  from: string;
  to: string;
  calculation_notes: string[];
  dashboard_url: string;
}

const stageLabels: Record<string, string> = {
  leads: "New enquiries",
  applications_started: "Applications started",
  applications_submitted: "Applications submitted",
  admitted: "Admitted",
  offers_issued: "Offers issued",
  offers_accepted: "Offers accepted",
};

function percent(value: number | null) {
  return value === null
    ? "Not enough data"
    : `${value.toLocaleString("en-GB", { maximumFractionDigits: 1 })}%`;
}

function sourceLabel(value: string) {
  return value.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export default async function GrowthAnalyticsPage({
  searchParams,
}: {
  searchParams: Promise<{ from?: string; to?: string; question?: string }>;
}) {
  await requireAuth();
  const query = await searchParams;
  const params = new URLSearchParams();
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  const client = await createServerClient();
  let report: Report | null = null;
  let error: string | null = null;
  let programmes: Programme[] = [];
  let executiveAnswer: ExecutiveAnswer | null = null;
  try {
    const [analytics, catalogue] = await Promise.all([
      client.get<Report>(`/api/v1/analytics/executive/growth${params.size ? `?${params}` : ""}`),
      client.get<{ data: Programme[] }>("/api/v1/programmes?limit=100").catch(() => ({ data: [] })),
    ]);
    report = analytics;
    programmes = catalogue.data;
    if (query.question?.trim())
      executiveAnswer = await client.post<ExecutiveAnswer>("/api/v1/analytics/executive/query", {
        question: query.question,
        from: report.from,
        to: report.to,
      });
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load Growth analytics";
  }

  const programmeNames = new Map(programmes.map((programme) => [programme.id, programme.name]));
  const leads = report?.funnel.find((step) => step.stage === "leads")?.count ?? 0;
  const accepted = report?.funnel.find((step) => step.stage === "offers_accepted")?.count ?? 0;
  const applicationRate =
    report?.funnel.find((step) => step.stage === "applications_started")?.conversion_from_lead ??
    null;
  const offerRate =
    report?.funnel.find((step) => step.stage === "offers_accepted")?.conversion_from_lead ?? null;

  return (
    <div className="space-y-7">
      <PageHeader
        icon={<BarChart3 className="size-7" />}
        title="Growth intelligence"
        description="A traceable view from first enquiry to accepted offer—attributed by source and programme, with every calculation exposed."
      />

      <form
        className="flex flex-col gap-3 rounded-2xl border border-border bg-surface p-4 sm:flex-row sm:items-end"
        method="get"
      >
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          From
          <input
            type="date"
            name="from"
            defaultValue={report?.from ?? query.from}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          To
          <input
            type="date"
            name="to"
            defaultValue={report?.to ?? query.to}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          />
        </label>
        <button
          type="submit"
          className="h-10 rounded-md bg-primary px-5 text-sm font-bold text-primary-foreground transition hover:-translate-y-0.5 hover:shadow-md"
        >
          Refresh window
        </button>
        {report ? (
          <p className="sm:ml-auto sm:pb-2 text-xs text-muted-foreground">
            Generated {new Date(report.generated_at).toLocaleString("en-GB")}
          </p>
        ) : null}
      </form>

      {error || !report ? (
        <EmptyState
          title="Growth analytics are unavailable"
          description={error ?? "No report was returned."}
          icon={<CircleAlert className="size-8" />}
        />
      ) : (
        <>
          <Reveal>
            <section className="rounded-2xl border border-border bg-surface p-5">
              <div className="flex items-start gap-3">
                <Sparkles className="mt-1 size-5 text-primary" />
                <div className="w-full">
                  <h2 className="font-heading text-xl font-bold">Ask the operating data</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Ask about the funnel, sources, programmes, or the 30-day forecast. Answers use
                    approved calculations only.
                  </p>
                  <form method="get" className="mt-4 flex flex-col gap-2 sm:flex-row">
                    <input type="hidden" name="from" value={report.from} />
                    <input type="hidden" name="to" value={report.to} />
                    <input
                      name="question"
                      defaultValue={query.question}
                      minLength={5}
                      maxLength={500}
                      required
                      placeholder="Which channel generated the most accepted offers?"
                      className="h-11 min-w-0 flex-1 rounded-md border border-border bg-background px-3 text-sm"
                    />
                    <button className="h-11 rounded-md bg-foreground px-5 text-sm font-bold text-background">
                      Ask question
                    </button>
                  </form>
                  {executiveAnswer ? (
                    <div className="mt-5 rounded-xl bg-muted p-4">
                      <div className="flex items-center justify-between gap-3">
                        <p className="text-xs font-bold uppercase tracking-wider text-primary">
                          Grounded answer
                        </p>
                        <span className="text-xs font-semibold">
                          {executiveAnswer.confidence} confidence
                        </span>
                      </div>
                      <p className="mt-2 font-semibold leading-7">{executiveAnswer.answer}</p>
                      <p className="mt-3 text-xs leading-5 text-muted-foreground">
                        Window: {executiveAnswer.from} → {executiveAnswer.to} · Sources:{" "}
                        {executiveAnswer.source_datasets.join(", ")} ·{" "}
                        {executiveAnswer.calculation_notes[0]}
                      </p>
                    </div>
                  ) : null}
                </div>
              </div>
            </section>
          </Reveal>
          <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Reveal>
              <StatCard label="New enquiries" value={leads.toLocaleString()} unit="in window" />
            </Reveal>
            <Reveal delay={60}>
              <StatCard
                label="Lead → application"
                value={percent(applicationRate)}
                unit="started"
                tone={applicationRate !== null && applicationRate >= 25 ? "ok" : "warn"}
              />
            </Reveal>
            <Reveal delay={120}>
              <StatCard
                label="Lead → accepted offer"
                value={percent(offerRate)}
                unit={`${accepted} accepted`}
                tone={offerRate !== null && offerRate >= 10 ? "ok" : "warn"}
              />
            </Reveal>
            <Reveal delay={180}>
              <StatCard
                label="Next 30 days"
                value={report.forecast.projected_offer_acceptances.toLocaleString()}
                unit={`accepted offers · ${report.forecast.confidence} confidence`}
              />
            </Reveal>
          </section>

          <Reveal delay={100}>
            <section className="overflow-hidden rounded-3xl border border-border bg-surface shadow-sm">
              <div className="flex flex-col gap-3 border-b border-border bg-gradient-to-r from-primary/10 via-transparent to-secondary/10 p-6 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="text-xs font-extrabold uppercase tracking-[0.2em] text-primary">
                    Admissions conversion
                  </p>
                  <h2 className="mt-2 font-heading text-2xl font-bold">
                    The live recruitment funnel
                  </h2>
                </div>
                <div className="rounded-full border border-border bg-background/80 px-4 py-2 text-xs font-semibold text-muted-foreground">
                  {report.from} → {report.to}
                </div>
              </div>
              <div className="grid gap-0 lg:grid-cols-[1fr_19rem]">
                <div className="space-y-5 p-6">
                  {report.funnel.map((step, index) => {
                    const width =
                      leads > 0 ? Math.max(4, Math.min(100, (step.count / leads) * 100)) : 4;
                    return (
                      <div
                        key={step.stage}
                        className="group grid gap-2 sm:grid-cols-[12rem_1fr_7rem] sm:items-center"
                      >
                        <div>
                          <p className="text-sm font-bold">{stageLabels[step.stage]}</p>
                          <p className="text-xs text-muted-foreground">
                            {index === 0
                              ? "Funnel entry"
                              : `${percent(step.conversion_from_previous)} from previous`}
                          </p>
                        </div>
                        <div className="h-3 overflow-hidden rounded-full bg-muted">
                          <div
                            className="h-full rounded-full bg-gradient-to-r from-primary to-secondary transition-all duration-700 group-hover:brightness-110"
                            style={{ width: `${width}%` }}
                          />
                        </div>
                        <p className="text-right font-heading text-2xl font-black tabular-nums">
                          {step.count.toLocaleString()}
                        </p>
                      </div>
                    );
                  })}
                </div>
                <aside className="border-t border-border bg-foreground p-6 text-background lg:border-l lg:border-t-0">
                  <Sparkles className="size-7 text-secondary" />
                  <p className="mt-6 text-xs font-bold uppercase tracking-[0.18em] text-background/60">
                    Operating forecast
                  </p>
                  <p className="mt-3 font-heading text-5xl font-black">
                    {report.forecast.projected_offer_acceptances.toLocaleString()}
                  </p>
                  <p className="mt-2 text-sm leading-6 text-background/70">
                    projected accepted offers over the next {report.forecast.horizon_days} days.
                  </p>
                  <div className="mt-6 rounded-xl border border-background/15 bg-background/5 p-4 text-xs leading-5 text-background/70">
                    <strong className="text-background">
                      {sourceLabel(report.forecast.confidence)} confidence.
                    </strong>{" "}
                    Based on {report.forecast.observed_days} observed days. This is a transparent
                    run-rate, not an ML prediction.
                  </div>
                </aside>
              </div>
            </section>
          </Reveal>

          <section className="grid gap-6 xl:grid-cols-2">
            <BreakdownPanel
              icon={<Waypoints className="size-5" />}
              title="Source performance"
              description="See which channels create intent—and which carry it through to an accepted offer."
              rows={report.by_source}
              label={sourceLabel}
            />
            <BreakdownPanel
              icon={<Target className="size-5" />}
              title="Programme demand"
              description="Compare application volume and offer outcomes without exposing applicant identity."
              rows={report.by_programme}
              label={(key) => programmeNames.get(key) ?? `Programme ${key.slice(0, 8)}`}
              hideLeads
            />
          </section>

          <section className="rounded-2xl border border-border bg-muted/40 p-5">
            <div className="flex gap-3">
              <Activity className="mt-0.5 size-5 text-primary" />
              <div>
                <h2 className="font-bold">Calculation and data-quality notes</h2>
                <ul className="mt-2 space-y-1 text-sm leading-6 text-muted-foreground">
                  {report.forecast.calculation_notes.map((note) => (
                    <li key={note}>• {note}</li>
                  ))}
                </ul>
                {report.data_quality.unattributed_application_events > 0 ? (
                  <p className="mt-3 flex items-center gap-2 rounded-lg bg-amber-100 px-3 py-2 text-sm font-semibold text-amber-900">
                    <CircleAlert className="size-4" />
                    {report.data_quality.unattributed_application_events} application events could
                    not be linked to a lead source.
                  </p>
                ) : (
                  <p className="mt-3 text-sm font-semibold text-emerald-700">
                    All application events in this window have source attribution.
                  </p>
                )}
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function BreakdownPanel({
  icon,
  title,
  description,
  rows,
  label,
  hideLeads = false,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  rows: Breakdown[];
  label: (key: string) => string;
  hideLeads?: boolean;
}) {
  return (
    <Reveal variant="up">
      <section className="h-full rounded-2xl border border-border bg-surface p-6">
        <div className="flex items-start gap-3">
          <span className="rounded-xl bg-primary/10 p-2 text-primary">{icon}</span>
          <div>
            <h2 className="font-heading text-xl font-bold">{title}</h2>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">{description}</p>
          </div>
        </div>
        {rows.length === 0 ? (
          <p className="mt-8 rounded-xl bg-muted p-5 text-sm text-muted-foreground">
            No attributed events in this reporting window.
          </p>
        ) : (
          <div className="mt-6 divide-y divide-border">
            {rows.slice(0, 8).map((row) => (
              <article
                key={row.key}
                className="grid gap-3 py-4 sm:grid-cols-[1fr_auto] sm:items-center"
              >
                <div className="min-w-0">
                  <h3 className="truncate font-bold">{label(row.key)}</h3>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {hideLeads
                      ? `${row.applications_started} applications started`
                      : `${row.leads} leads · ${percent(row.lead_to_application_rate)} started an application`}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <div className="text-right">
                    <p className="font-heading text-xl font-black">{row.offers_accepted}</p>
                    <p className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground">
                      accepted
                    </p>
                  </div>
                  <ArrowDownRight className="size-4 text-primary" />
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </Reveal>
  );
}
