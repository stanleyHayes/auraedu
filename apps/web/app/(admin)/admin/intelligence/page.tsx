import { revalidatePath } from "next/cache";
import { headers } from "next/headers";
import { BadgeCheck, Fingerprint, Globe2, RadioTower, Scale, ShieldAlert } from "lucide-react";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { FeatureDisabled } from "@auraedu/flags";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { isFeatureEnabled } from "@/lib/features";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

type Kind = "reputation" | "competitor";
type Status = "pending_review" | "approved" | "rejected" | "resolved";
interface Source {
  id: string;
  kind: Kind;
  name: string;
  canonical_url: string;
  collection_method: "manual" | "official_api";
  terms_reference: string;
  compliance_status: Status;
  created_by: string;
  reviewed_by: string | null;
  review_note: string | null;
  created_at: string;
}
interface Observation {
  id: string;
  source_id: string;
  kind: Kind;
  category: string;
  title: string;
  evidence_excerpt: string;
  evidence_sha256: string;
  sentiment: string;
  response_draft: string;
  status: Status;
  created_by: string;
  observed_at: string;
  review_note: string | null;
  resolution_note: string | null;
}
interface AlertRule {
  threshold: number;
  window_days: number;
  updated_by: string;
  updated_at: string;
}
interface Alert {
  id: string;
  category: "recurring_issue" | "misinformation";
  programme_id: string | null;
  campus_id: string | null;
  observation_count: number;
  threshold: number;
  window_days: number;
  first_observed_at: string;
  last_observed_at: string;
  reason: string;
  status: "open" | "acknowledged";
  acknowledgement_note: string | null;
}
interface SummaryItem {
  source_id: string;
  category: string;
  change_type: "first_seen" | "changed";
  latest_title: string;
  latest_excerpt: string;
  latest_observed_at: string;
  previous_excerpt: string | null;
  previous_observed_at: string | null;
}
interface CompetitorSummary {
  id: string;
  period_from: string;
  period_to: string;
  status: "pending_review" | "approved" | "rejected";
  items: SummaryItem[];
  item_count: number;
  source_count: number;
  review_note: string | null;
}

function text(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}
function page(kind: Kind) {
  return `/admin/intelligence?kind=${kind}`;
}
async function createSource(data: FormData) {
  "use server";
  const kind = text(data, "kind") as Kind;
  const client = await createServerClient();
  await client.post("/api/v1/intelligence/sources", {
    kind,
    name: text(data, "name"),
    canonical_url: text(data, "canonical_url"),
    collection_method: text(data, "collection_method"),
    terms_reference: text(data, "terms_reference"),
  });
  revalidatePath(page(kind));
}
async function reviewSource(data: FormData) {
  "use server";
  const kind = text(data, "kind") as Kind;
  const client = await createServerClient();
  await client.post(`/api/v1/intelligence/sources/${text(data, "id")}/review`, {
    decision: text(data, "decision"),
    review_note: text(data, "review_note"),
  });
  revalidatePath(page(kind));
}
async function createObservation(data: FormData) {
  "use server";
  const kind = text(data, "kind") as Kind;
  const observed = new Date(text(data, "observed_at"));
  if (!Number.isFinite(observed.getTime())) return;
  const client = await createServerClient();
  await client.post("/api/v1/intelligence/observations", {
    source_id: text(data, "source_id"),
    category: text(data, "category"),
    title: text(data, "title"),
    evidence_excerpt: text(data, "evidence_excerpt"),
    sentiment: text(data, "sentiment"),
    programme_id: null,
    campus_id: null,
    response_draft: text(data, "response_draft"),
    observed_at: observed.toISOString(),
  });
  revalidatePath(page(kind));
}
async function reviewObservation(data: FormData) {
  "use server";
  const kind = text(data, "kind") as Kind;
  const client = await createServerClient();
  await client.post(`/api/v1/intelligence/observations/${text(data, "id")}/review`, {
    decision: text(data, "decision"),
    review_note: text(data, "review_note"),
  });
  revalidatePath(page(kind));
}
async function resolveObservation(data: FormData) {
  "use server";
  const kind = text(data, "kind") as Kind;
  const client = await createServerClient();
  await client.post(`/api/v1/intelligence/observations/${text(data, "id")}/resolve`, {
    resolution_note: text(data, "resolution_note"),
  });
  revalidatePath(page(kind));
}
async function updateAlertRule(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.put("/api/v1/intelligence/alert-rule", {
    threshold: Number(text(data, "threshold")),
    window_days: Number(text(data, "window_days")),
  });
  revalidatePath(page("reputation"));
}
async function acknowledgeAlert(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.post(`/api/v1/intelligence/alerts/${text(data, "id")}/acknowledge`, {
    acknowledgement_note: text(data, "acknowledgement_note"),
  });
  revalidatePath(page("reputation"));
}
async function generateSummary(data: FormData) {
  "use server";
  const from = new Date(text(data, "period_from"));
  const to = new Date(text(data, "period_to"));
  if (!Number.isFinite(from.getTime()) || !Number.isFinite(to.getTime())) return;
  const client = await createServerClient();
  await client.post("/api/v1/intelligence/competitor-summaries", {
    period_from: from.toISOString(),
    period_to: to.toISOString(),
  });
  revalidatePath(page("competitor"));
}
async function reviewSummary(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.post(`/api/v1/intelligence/competitor-summaries/${text(data, "id")}/review`, {
    decision: text(data, "decision"),
    review_note: text(data, "review_note"),
  });
  revalidatePath(page("competitor"));
}

const categoryOptions: { value: string; label: string; kind: Kind }[] = [
  { value: "mention", label: "Public mention", kind: "reputation" },
  { value: "recurring_issue", label: "Recurring issue", kind: "reputation" },
  { value: "misinformation", label: "Misinformation", kind: "reputation" },
  { value: "programme", label: "Programme offering", kind: "competitor" },
  { value: "fee", label: "Fee", kind: "competitor" },
  { value: "scholarship", label: "Scholarship", kind: "competitor" },
  { value: "deadline", label: "Deadline", kind: "competitor" },
  { value: "campaign", label: "Campaign", kind: "competitor" },
];

export default async function IntelligencePage({
  searchParams,
}: {
  searchParams: Promise<{ kind?: string }>;
}) {
  await requireAuth();
  const query = await searchParams;
  const kind: Kind = query.kind === "competitor" ? "competitor" : "reputation";
  const feature = kind === "competitor" ? "growth_competitor_monitor" : "growth_reputation_monitor";
  const requestHeaders = await headers();
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(requestHeaders));
  if (!isFeatureEnabled(tenant.features, feature)) {
    return <FeatureDisabled feature={feature} />;
  }
  let sources: Source[] = [];
  let observations: Observation[] = [];
  let alerts: Alert[] = [];
  let summaries: CompetitorSummary[] = [];
  let alertRule: AlertRule | null = null;
  let error: string | null = null;
  try {
    const client = await createServerClient();
    [sources, observations] = await Promise.all([
      client
        .get<{ data: Source[] }>(`/api/v1/intelligence/sources?kind=${kind}&limit=100`)
        .then((v) => v.data),
      client
        .get<{ data: Observation[] }>(`/api/v1/intelligence/observations?kind=${kind}&limit=100`)
        .then((v) => v.data),
    ]);
    if (kind === "reputation") {
      [alerts, alertRule] = await Promise.all([
        client.get<{ data: Alert[] }>("/api/v1/intelligence/alerts?limit=100").then((v) => v.data),
        client.get<AlertRule>("/api/v1/intelligence/alert-rule"),
      ]);
    } else {
      summaries = await client
        .get<{ data: CompetitorSummary[] }>("/api/v1/intelligence/competitor-summaries?limit=50")
        .then((v) => v.data);
    }
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load intelligence workspace";
  }
  const approved = sources.filter((v) => v.compliance_status === "approved");
  const queue = observations.filter((v) => v.status === "pending_review").length;
  const openAlerts = alerts.filter((v) => v.status === "open");
  const title = kind === "reputation" ? "Reputation desk" : "Competitor watch";
  const description =
    kind === "reputation"
      ? "Turn public signals into reviewed, evidence-backed issues—then coordinate a human response without auto-posting."
      : "Maintain a lawful market record from manual research and official APIs. No uncontrolled scraping, no copied pages.";
  return (
    <div className="space-y-6">
      <PageHeader
        icon={
          kind === "reputation" ? <RadioTower className="size-7" /> : <Globe2 className="size-7" />
        }
        title={title}
        description={description}
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Approved sources" value={approved.length} />
        <StatCard label="Awaiting human review" value={queue} />
        <StatCard
          label={kind === "reputation" ? "Open threshold alerts" : "Verified observations"}
          value={
            kind === "reputation"
              ? openAlerts.length
              : observations.filter((v) => v.status === "approved").length
          }
        />
      </div>
      {kind === "reputation" && alertRule ? (
        <section className="rounded-2xl border border-border bg-surface p-6">
          <div className="grid gap-6 lg:grid-cols-[1fr_auto]">
            <div>
              <div className="flex items-center gap-3">
                <ShieldAlert className="size-5 text-primary" />
                <h2 className="font-heading text-xl font-bold">Explainable issue alerts</h2>
              </div>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
                AuraEDU groups independently approved recurring-issue or misinformation records by
                category, programme and campus. An alert opens only when the visible count reaches
                your visible time-window rule—no hidden model score.
              </p>
            </div>
            <form
              action={updateAlertRule}
              className="grid grid-cols-[6rem_6rem_auto] items-end gap-2"
            >
              <label className="text-xs font-bold">
                Count
                <input
                  required
                  type="number"
                  min="2"
                  max="20"
                  name="threshold"
                  defaultValue={alertRule.threshold}
                  className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal"
                />
              </label>
              <label className="text-xs font-bold">
                Days
                <input
                  required
                  type="number"
                  min="1"
                  max="90"
                  name="window_days"
                  defaultValue={alertRule.window_days}
                  className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal"
                />
              </label>
              <Button type="submit" variant="secondary">
                Update rule
              </Button>
            </form>
          </div>
          {alerts.length > 0 ? (
            <div className="mt-6 grid gap-3">
              {alerts.map((alert) => (
                <article
                  key={alert.id}
                  className={`rounded-xl border p-4 ${alert.status === "open" ? "border-amber-300 bg-amber-50/60" : "border-border bg-muted/40"}`}
                >
                  <div className="flex flex-col justify-between gap-3 sm:flex-row">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <StatusPill
                          status={alert.status === "open" ? "pending_review" : "resolved"}
                        />
                        <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                          {alert.category.replaceAll("_", " ")}
                        </span>
                      </div>
                      <h3 className="mt-2 font-bold">{alert.reason}</h3>
                      <p className="mt-1 text-xs text-muted-foreground">
                        First {new Date(alert.first_observed_at).toLocaleString("en-GB")} · latest{" "}
                        {new Date(alert.last_observed_at).toLocaleString("en-GB")}
                      </p>
                    </div>
                    {alert.status === "open" ? (
                      <form action={acknowledgeAlert} className="flex min-w-0 gap-2">
                        <input type="hidden" name="id" value={alert.id} />
                        <input
                          required
                          minLength={3}
                          maxLength={1000}
                          name="acknowledgement_note"
                          placeholder="Owner and next step"
                          className="h-10 min-w-0 rounded-md border border-border bg-background px-3 text-sm"
                        />
                        <Button type="submit">Acknowledge</Button>
                      </form>
                    ) : (
                      <p className="text-sm text-muted-foreground">{alert.acknowledgement_note}</p>
                    )}
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <p className="mt-5 rounded-xl bg-muted p-4 text-sm text-muted-foreground">
              No threshold has been reached. The current rule opens an alert at{" "}
              {alertRule.threshold} matching approved observations within {alertRule.window_days}{" "}
              days.
            </p>
          )}
        </section>
      ) : null}
      {kind === "competitor" ? (
        <section className="rounded-2xl border border-border bg-surface p-6">
          <div className="grid gap-6 lg:grid-cols-[1fr_auto]">
            <div>
              <div className="flex items-center gap-3">
                <Fingerprint className="size-5 text-primary" />
                <h2 className="font-heading text-xl font-bold">Versioned market brief</h2>
              </div>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
                Compare only approved evidence versions. Each item shows the prior and latest
                bounded excerpt, timestamps and evidence hashes; generation never fetches a page or
                reproduces full copyrighted content.
              </p>
            </div>
            <form
              action={generateSummary}
              className="grid grid-cols-[10rem_10rem_auto] items-end gap-2"
            >
              <label className="text-xs font-bold">
                From
                <input
                  required
                  type="date"
                  name="period_from"
                  defaultValue={new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10)}
                  className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal"
                />
              </label>
              <label className="text-xs font-bold">
                To
                <input
                  required
                  type="date"
                  name="period_to"
                  defaultValue={new Date().toISOString().slice(0, 10)}
                  className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal"
                />
              </label>
              <Button type="submit">Generate draft</Button>
            </form>
          </div>
          {summaries.length === 0 ? (
            <p className="mt-5 rounded-xl bg-muted p-4 text-sm text-muted-foreground">
              No briefs yet. Record and approve competitor observations, then generate a reviewable
              comparison for a bounded period.
            </p>
          ) : (
            <div className="mt-6 grid gap-4">
              {summaries.map((summary) => (
                <article key={summary.id} className="rounded-xl border border-border p-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <div className="flex items-center gap-2">
                        <StatusPill status={summary.status} />
                        <span className="text-xs text-muted-foreground">
                          {summary.item_count} changes · {summary.source_count} sources
                        </span>
                      </div>
                      <h3 className="mt-2 font-bold">
                        Market brief · {new Date(summary.period_from).toLocaleDateString("en-GB")}–
                        {new Date(summary.period_to).toLocaleDateString("en-GB")}
                      </h3>
                    </div>
                  </div>
                  <div className="mt-4 grid gap-3 lg:grid-cols-2">
                    {summary.items.map((item, index) => (
                      <div
                        key={`${item.source_id}-${item.category}-${index}`}
                        className="rounded-xl bg-muted p-4"
                      >
                        <p className="text-xs font-extrabold uppercase tracking-wider text-primary">
                          {item.change_type.replace("_", " ")} · {item.category}
                        </p>
                        <h4 className="mt-2 font-bold">{item.latest_title}</h4>
                        {item.previous_excerpt ? (
                          <div className="mt-3 border-l-2 border-border pl-3">
                            <p className="text-xs font-bold text-muted-foreground">
                              Previous ·{" "}
                              {item.previous_observed_at
                                ? new Date(item.previous_observed_at).toLocaleDateString("en-GB")
                                : "earlier"}
                            </p>
                            <p className="mt-1 text-sm text-muted-foreground">
                              {item.previous_excerpt}
                            </p>
                          </div>
                        ) : null}
                        <div className="mt-3 border-l-2 border-primary pl-3">
                          <p className="text-xs font-bold text-primary">
                            Latest · {new Date(item.latest_observed_at).toLocaleDateString("en-GB")}
                          </p>
                          <p className="mt-1 text-sm">{item.latest_excerpt}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                  {summary.status === "pending_review" ? (
                    <ReviewForm
                      action={reviewSummary}
                      id={summary.id}
                      kind="competitor"
                      label="Approve market brief"
                    />
                  ) : summary.review_note ? (
                    <p className="mt-4 text-sm text-muted-foreground">
                      Review: {summary.review_note}
                    </p>
                  ) : null}
                </article>
              ))}
            </div>
          )}
        </section>
      ) : null}
      <section className="overflow-hidden rounded-2xl border border-border bg-surface">
        <div className="grid lg:grid-cols-[0.9fr_1.1fr]">
          <div className="bg-[radial-gradient(circle_at_top_left,hsl(var(--primary)/0.18),transparent_58%)] p-6 lg:p-8">
            <div className="flex size-11 items-center justify-center rounded-2xl bg-primary text-primary-foreground">
              <Scale className="size-5" />
            </div>
            <h2 className="mt-5 font-heading text-2xl font-black">Register the authority first</h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
              Every source begins in compliance review. Record why collection is lawful; a different
              authorised person must approve it before evidence can enter the workspace.
            </p>
            <div className="mt-6 rounded-xl border border-border/70 bg-background/70 p-4 text-xs leading-5 text-muted-foreground">
              <strong className="text-foreground">Collection boundary</strong>
              <br />
              Only manual research and official APIs are accepted. AuraEDU does not bypass terms,
              robots directives, paywalls, authentication, or rate limits.
            </div>
          </div>
          <form action={createSource} className="grid gap-4 p-6 lg:p-8">
            <input type="hidden" name="kind" value={kind} />
            <label className="text-sm font-semibold">
              Source name
              <input
                required
                minLength={3}
                maxLength={160}
                name="name"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
                placeholder={
                  kind === "reputation"
                    ? "Official Google Business profile"
                    : "University programme catalogue"
                }
              />
            </label>
            <label className="text-sm font-semibold">
              Canonical public URL
              <input
                required
                type="url"
                maxLength={2048}
                name="canonical_url"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
                placeholder="https://…"
              />
            </label>
            <label className="text-sm font-semibold">
              Collection method
              <select
                name="collection_method"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              >
                <option value="manual">Human research</option>
                <option value="official_api">Official public API</option>
              </select>
            </label>
            <label className="text-sm font-semibold">
              Terms or authority reference
              <textarea
                required
                minLength={3}
                maxLength={1000}
                rows={3}
                name="terms_reference"
                className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
                placeholder="Link to API terms, documented permission, or approved manual research policy…"
              />
            </label>
            <Button type="submit">Send source to compliance review</Button>
          </form>
        </div>
      </section>
      <section className="space-y-4">
        <div>
          <h2 className="font-heading text-xl font-bold">Source compliance register</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Approval establishes collection authority; it does not certify every future observation.
          </p>
        </div>
        {error ? (
          <EmptyState
            title="Could not load intelligence"
            description={error}
            icon={<ShieldAlert className="size-8" />}
          />
        ) : sources.length === 0 ? (
          <EmptyState
            title="No sources registered"
            description="Start with an official page or a manual research source whose terms have been checked."
            icon={<Scale className="size-8" />}
          />
        ) : (
          <div className="grid gap-4 lg:grid-cols-2">
            {sources.map((source) => (
              <article key={source.id} className="rounded-xl border border-border bg-surface p-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <h3 className="truncate font-bold">{source.name}</h3>
                    <a
                      href={source.canonical_url}
                      target="_blank"
                      rel="noreferrer"
                      className="mt-1 block truncate text-xs text-primary hover:underline"
                    >
                      {source.canonical_url}
                    </a>
                  </div>
                  <StatusPill status={source.compliance_status} />
                </div>
                <p className="mt-4 text-sm leading-6 text-muted-foreground">
                  {source.terms_reference}
                </p>
                <div className="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
                  <Fingerprint className="size-3.5" />
                  {source.collection_method.replace("_", " ")}
                </div>
                {source.compliance_status === "pending_review" ? (
                  <ReviewForm
                    action={reviewSource}
                    id={source.id}
                    kind={kind}
                    label="Complete source review"
                  />
                ) : source.review_note ? (
                  <p className="mt-4 border-l-2 border-primary pl-3 text-xs text-muted-foreground">
                    {source.review_note}
                  </p>
                ) : null}
              </article>
            ))}
          </div>
        )}
      </section>
      {approved.length > 0 ? (
        <section className="rounded-2xl border border-border bg-surface p-6">
          <div className="mb-5 flex items-start gap-3">
            <ShieldAlert className="mt-0.5 size-5 text-primary" />
            <div>
              <h2 className="font-heading text-xl font-bold">Record bounded evidence</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Quote only the minimum excerpt needed for review. A SHA-256 fingerprint preserves
                evidence integrity without copying a full page.
              </p>
            </div>
          </div>
          <form action={createObservation} className="grid gap-4 md:grid-cols-2">
            <input type="hidden" name="kind" value={kind} />
            <label className="text-sm font-semibold">
              Approved source
              <select
                required
                name="source_id"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              >
                {approved.map((v) => (
                  <option key={v.id} value={v.id}>
                    {v.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-semibold">
              Category
              <select
                name="category"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              >
                {categoryOptions
                  .filter((v) => v.kind === kind)
                  .map((v) => (
                    <option key={v.value} value={v.value}>
                      {v.label}
                    </option>
                  ))}
              </select>
            </label>
            <label className="text-sm font-semibold md:col-span-2">
              Finding title
              <input
                required
                minLength={3}
                maxLength={240}
                name="title"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              />
            </label>
            <label className="text-sm font-semibold">
              Observed at
              <input
                required
                type="datetime-local"
                name="observed_at"
                defaultValue={new Date().toISOString().slice(0, 16)}
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              />
            </label>
            <label className="text-sm font-semibold">
              Sentiment
              <select
                name="sentiment"
                className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              >
                <option value="unknown">Not classified</option>
                <option value="positive">Positive</option>
                <option value="neutral">Neutral</option>
                <option value="negative">Negative</option>
              </select>
            </label>
            <label className="text-sm font-semibold md:col-span-2">
              Evidence excerpt
              <textarea
                required
                minLength={3}
                maxLength={1000}
                rows={4}
                name="evidence_excerpt"
                className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
              />
            </label>
            {kind === "reputation" ? (
              <label className="text-sm font-semibold md:col-span-2">
                Internal response draft (optional)
                <textarea
                  maxLength={4000}
                  rows={4}
                  name="response_draft"
                  className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
                />
                <span className="mt-1 block text-xs font-normal text-muted-foreground">
                  Approval keeps this draft inside AuraEDU. There is intentionally no publish
                  action.
                </span>
              </label>
            ) : null}
            <div className="md:col-span-2">
              <Button type="submit">Save evidence for human review</Button>
            </div>
          </form>
        </section>
      ) : null}
      <section className="space-y-4">
        <div>
          <h2 className="font-heading text-xl font-bold">Evidence review queue</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Every card preserves its source, observation time, evidence fingerprint, and independent
            decision trail.
          </p>
        </div>
        {observations.length === 0 ? (
          <EmptyState
            title="No observations yet"
            description="Approve a lawful source, then record the first evidence-backed signal."
            icon={<RadioTower className="size-8" />}
          />
        ) : (
          observations.map((item) => (
            <article key={item.id} className="rounded-xl border border-border bg-surface p-5">
              <div className="flex flex-col justify-between gap-4 md:flex-row">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-bold">{item.title}</h3>
                    <StatusPill status={item.status} />
                    <span className="rounded-full border border-border px-2.5 py-1 text-xs">
                      {item.category.replaceAll("_", " ")}
                    </span>
                  </div>
                  <p className="mt-3 max-w-3xl text-sm leading-6 text-muted-foreground">
                    {item.evidence_excerpt}
                  </p>
                  <p className="mt-3 font-mono text-[0.65rem] text-muted-foreground">
                    SHA-256 {item.evidence_sha256}
                  </p>
                  {item.response_draft ? (
                    <div className="mt-4 rounded-xl bg-muted p-4">
                      <p className="text-xs font-extrabold uppercase tracking-[0.16em] text-primary">
                        Internal response draft · never auto-published
                      </p>
                      <p className="mt-2 text-sm leading-6">{item.response_draft}</p>
                    </div>
                  ) : null}
                </div>
              </div>
              {item.status === "pending_review" ? (
                <ReviewForm
                  action={reviewObservation}
                  id={item.id}
                  kind={kind}
                  label="Review evidence and draft"
                />
              ) : null}
              {item.status === "approved" && kind === "reputation" ? (
                <form
                  action={resolveObservation}
                  className="mt-4 flex flex-col gap-2 rounded-xl border border-border p-4 sm:flex-row"
                >
                  <input type="hidden" name="id" value={item.id} />
                  <input type="hidden" name="kind" value={kind} />
                  <input
                    required
                    minLength={3}
                    maxLength={2000}
                    name="resolution_note"
                    placeholder="How was the issue resolved internally?"
                    className="h-10 min-w-0 flex-1 rounded-md border border-border bg-background px-3 text-sm"
                  />
                  <Button type="submit" variant="secondary">
                    Mark resolved
                  </Button>
                </form>
              ) : null}
            </article>
          ))
        )}
      </section>
    </div>
  );
}

function StatusPill({ status }: { status: Status }) {
  const tone =
    status === "approved" || status === "resolved"
      ? "bg-emerald-50 text-emerald-800"
      : status === "pending_review"
        ? "bg-amber-50 text-amber-800"
        : "bg-rose-50 text-rose-800";
  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-bold ${tone}`}>
      {status.replaceAll("_", " ")}
    </span>
  );
}
function ReviewForm({
  action,
  id,
  kind,
  label,
}: {
  action: (data: FormData) => Promise<void>;
  id: string;
  kind: Kind;
  label: string;
}) {
  return (
    <form
      action={action}
      className="mt-4 grid gap-2 border-t border-border pt-4 sm:grid-cols-[1fr_auto_auto]"
    >
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="kind" value={kind} />
      <input
        required
        minLength={3}
        maxLength={1000}
        name="review_note"
        placeholder="What did you independently verify?"
        className="h-10 rounded-md border border-border bg-background px-3 text-sm"
      />
      <Button type="submit" name="decision" value="approved">
        <BadgeCheck className="mr-2 size-4" />
        {label}
      </Button>
      <Button type="submit" name="decision" value="rejected" variant="secondary">
        Reject
      </Button>
    </form>
  );
}
