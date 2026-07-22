import { revalidatePath } from "next/cache";
import { Archive, Clock3, GitBranch, PauseCircle, PlayCircle, ShieldCheck } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import { JourneyBuilder } from "@/components/journey-builder";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Journey = OpenAPI.notification_v1.components["schemas"]["CommunicationJourney"];
type JourneyStats = OpenAPI.notification_v1.components["schemas"]["JourneyStats"];
type Template = OpenAPI.notification_v1.components["schemas"]["Template"];

const emptyStats: JourneyStats = {
  enrolled: 0,
  scheduled: 0,
  sent: 0,
  failed: 0,
  cancelled: 0,
  skipped: 0,
  accepted: 0,
  delivered: 0,
  delayed: 0,
  bounced: 0,
  complained: 0,
  suppressed: 0,
};

function text(formData: FormData, key: string) {
  const value = formData.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function texts(formData: FormData, key: string) {
  return formData.getAll(key).map((value) => (typeof value === "string" ? value.trim() : ""));
}

function minuteOfDay(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  return hour <= 23 && minute <= 59 ? hour * 60 + minute : null;
}

async function createJourney(formData: FormData) {
  "use server";
  const channels = texts(formData, "step_channel");
  const templateIDs = texts(formData, "step_template_id");
  const delays = texts(formData, "step_delay_minutes");
  const operators = texts(formData, "step_condition_operator");
  const fields = texts(formData, "step_condition_field");
  const values = texts(formData, "step_condition_value");
  if (
    channels.length === 0 ||
    ![templateIDs.length, delays.length, operators.length, fields.length, values.length].every(
      (length) => length === channels.length,
    )
  )
    return;

  const quietStart = minuteOfDay(text(formData, "quiet_start"));
  const quietEnd = minuteOfDay(text(formData, "quiet_end"));
  const client = await createServerClient();
  await client.post("/api/v1/communication-journeys", {
    name: text(formData, "name"),
    trigger_event: text(formData, "trigger_event"),
    timezone: text(formData, "timezone"),
    quiet_hours_start_minute: quietStart,
    quiet_hours_end_minute: quietEnd,
    frequency_window_hours: Number(text(formData, "frequency_window_hours")),
    frequency_limit: Number(text(formData, "frequency_limit")),
    cancel_on_events: texts(formData, "cancel_on_events"),
    steps: channels.map((channel, index) => ({
      channel,
      template_id: templateIDs[index],
      delay_minutes: Number(delays[index]),
      condition_operator: operators[index],
      condition_field: fields[index]?.length ? fields[index] : undefined,
      condition_value: values[index]?.length ? values[index] : undefined,
    })),
  });
  revalidatePath("/admin/journeys");
}

async function transitionJourney(formData: FormData) {
  "use server";
  const id = text(formData, "id");
  const action = text(formData, "action");
  if (!id || !["activate", "pause", "archive"].includes(action)) return;
  const client = await createServerClient();
  await client.post(`/api/v1/communication-journeys/${id}/${action}`, {});
  revalidatePath("/admin/journeys");
}

export default async function JourneysPage() {
  await requireAuth();
  let journeys: Journey[] = [];
  let templates: Template[] = [];
  let error: string | null = null;
  let templateError: string | null = null;
  const client = await createServerClient();
  const [journeyResult, templateResult] = await Promise.allSettled([
    client.get<{ data: Journey[] }>("/api/v1/communication-journeys?limit=100"),
    client.get<{ data: Template[] }>("/api/v1/notification-templates?status=active&limit=100"),
  ]);
  if (journeyResult.status === "fulfilled") {
    journeys = journeyResult.value.data;
  } else {
    error =
      journeyResult.reason instanceof Error
        ? journeyResult.reason.message
        : "Failed to load communication journeys";
  }
  if (templateResult.status === "fulfilled") {
    templates = templateResult.value.data;
  } else {
    templateError =
      templateResult.reason instanceof Error
        ? templateResult.reason.message
        : "Failed to load approved templates";
  }

  const statsEntries = await Promise.all(
    journeys.map(async (journey) => {
      try {
        return [
          journey.id,
          await client.get<JourneyStats>(`/api/v1/communication-journeys/${journey.id}/stats`),
        ] as const;
      } catch {
        return [journey.id, emptyStats] as const;
      }
    }),
  );
  const stats = new Map(statsEntries);
  const active = journeys.filter((journey) => journey.status === "active").length;
  const awaitingReview = journeys.filter((journey) => journey.status === "draft").length;
  const delivered = statsEntries.reduce((sum, [, value]) => sum + value.delivered, 0);

  return (
    <div className="space-y-7">
      <PageHeader
        icon={<GitBranch className="size-7" />}
        title="Communication journeys"
        description="Build consent-aware admissions follow-up with deliberate timing, deterministic branches, quiet hours, cancellation rules, and an independent activation boundary."
      />

      <Reveal className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Active journeys" value={active} />
        <StatCard label="Awaiting review" value={awaitingReview} />
        <StatCard label="Provider delivered" value={delivered} />
      </Reveal>

      <Reveal delay={70}>
        <section className="relative overflow-hidden rounded-2xl border border-border bg-surface p-6 shadow-sm lg:p-8">
          <div className="pointer-events-none absolute -right-24 -top-24 size-64 rounded-full bg-primary/10 blur-3xl" />
          <div className="relative mb-6 flex items-start gap-3">
            <span className="grid size-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
              <Clock3 className="size-5" />
            </span>
            <div>
              <h2 className="font-heading text-2xl font-bold">Design a journey</h2>
              <p className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
                Use active, approved templates. AuraEDU stores only allowlisted event context,
                resolves the current lead contact privately, and checks consent again immediately
                before delivery.
              </p>
            </div>
          </div>
          {templateError || templates.length === 0 ? (
            <div className="mb-5 rounded-xl border border-amber-300/60 bg-amber-50 p-4 text-sm text-amber-950">
              {templateError
                ? `Approved templates are unavailable: ${templateError}. Existing journeys remain visible below.`
                : "Create at least one active notification template before saving a journey."}
            </div>
          ) : null}
          <JourneyBuilder
            templates={templates.map((template) => ({
              id: template.id,
              name: template.name,
              channel: template.channel,
            }))}
            action={createJourney}
          />
        </section>
      </Reveal>

      <Reveal delay={120} className="space-y-4">
        <div className="flex items-end justify-between gap-4">
          <div>
            <h2 className="font-heading text-2xl font-bold">Journey register</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Monitor delivery outcomes and pause the automation without losing its review trail.
            </p>
          </div>
          <div className="hidden items-center gap-2 text-xs font-semibold text-muted-foreground sm:flex">
            <ShieldCheck className="size-4 text-primary" /> Independently activated
          </div>
        </div>
        {error ? (
          <EmptyState
            title="Could not load journeys"
            description={error}
            icon={<GitBranch className="size-8" />}
          />
        ) : journeys.length === 0 ? (
          <EmptyState
            title="No journeys yet"
            description="Save a draft above, then ask another authorised person to review and activate it."
            icon={<GitBranch className="size-8" />}
          />
        ) : (
          <div className="grid gap-4 xl:grid-cols-2">
            {journeys.map((journey) => {
              const journeyStats = stats.get(journey.id) ?? emptyStats;
              return (
                <article
                  key={journey.id}
                  className="group rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-5 shadow-sm transition hover:-translate-y-0.5 hover:border-primary/35 hover:shadow-md"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="truncate font-heading text-lg font-bold">{journey.name}</h3>
                        <Status status={journey.status} />
                      </div>
                      <p className="mt-2 text-sm text-muted-foreground">
                        {labelForEvent(journey.trigger_event)} · {journey.steps?.length ?? 0} step
                        {journey.steps?.length === 1 ? "" : "s"}
                      </p>
                    </div>
                    <span className="rounded-lg bg-primary/10 px-2.5 py-1 text-xs font-bold text-primary">
                      v{journey.version}
                    </span>
                  </div>
                  <div className="mt-5 grid grid-cols-2 gap-2 sm:grid-cols-3">
                    <Metric label="Enrolled" value={journeyStats.enrolled} />
                    <Metric label="Accepted" value={journeyStats.accepted} />
                    <Metric label="Delivered" value={journeyStats.delivered} />
                    <Metric label="Pending" value={journeyStats.scheduled} />
                    <Metric label="Delayed" value={journeyStats.delayed} />
                    <Metric
                      label="Bounced"
                      value={journeyStats.bounced}
                      danger={journeyStats.bounced > 0}
                    />
                    <Metric
                      label="Complaints"
                      value={journeyStats.complained}
                      danger={journeyStats.complained > 0}
                    />
                    <Metric
                      label="Suppressed"
                      value={journeyStats.suppressed}
                      danger={journeyStats.suppressed > 0}
                    />
                    <Metric label="Skipped" value={journeyStats.skipped} />
                    <Metric label="Cancelled" value={journeyStats.cancelled} />
                    <Metric
                      label="Failed"
                      value={journeyStats.failed}
                      danger={journeyStats.failed > 0}
                    />
                  </div>
                  <div className="mt-5 flex flex-wrap gap-2 border-t border-border pt-4">
                    {journey.status === "draft" || journey.status === "paused" ? (
                      <JourneyAction
                        id={journey.id}
                        action="activate"
                        label={
                          journey.status === "draft" ? "Review and activate" : "Resume journey"
                        }
                        icon={<PlayCircle className="size-4" />}
                      />
                    ) : null}
                    {journey.status === "active" ? (
                      <JourneyAction
                        id={journey.id}
                        action="pause"
                        label="Pause"
                        icon={<PauseCircle className="size-4" />}
                        secondary
                      />
                    ) : null}
                    {journey.status !== "archived" ? (
                      <JourneyAction
                        id={journey.id}
                        action="archive"
                        label="Archive"
                        icon={<Archive className="size-4" />}
                        secondary
                      />
                    ) : null}
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </Reveal>
    </div>
  );
}

function JourneyAction({
  id,
  action,
  label,
  icon,
  secondary = false,
}: {
  id: string;
  action: string;
  label: string;
  icon: React.ReactNode;
  secondary?: boolean;
}) {
  return (
    <form action={transitionJourney}>
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="action" value={action} />
      <Button type="submit" variant={secondary ? "secondary" : "primary"} className="gap-2">
        {icon}
        {label}
      </Button>
    </form>
  );
}

function Metric({
  label,
  value,
  danger = false,
}: {
  label: string;
  value: number;
  danger?: boolean;
}) {
  return (
    <div
      className={`rounded-lg border px-3 py-2 ${danger ? "border-red-200 bg-red-50 text-red-900" : "border-border bg-background/70"}`}
    >
      <div className="text-lg font-bold">{value}</div>
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
    </div>
  );
}

function Status({ status }: { status: Journey["status"] }) {
  const style =
    status === "active"
      ? "bg-emerald-50 text-emerald-800"
      : status === "draft"
        ? "bg-amber-50 text-amber-800"
        : status === "paused"
          ? "bg-blue-50 text-blue-800"
          : "bg-muted text-muted-foreground";
  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${style}`}>
      {status}
    </span>
  );
}

function labelForEvent(event: string) {
  return event.replace(".v1", "").replaceAll("_", " ").replaceAll(".", " · ");
}
