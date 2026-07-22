import { revalidatePath } from "next/cache";
import type { ReactNode } from "react";
import {
  AlertTriangle,
  ArrowRight,
  Bot,
  CheckCircle2,
  Fingerprint,
  LockKeyhole,
  RotateCcw,
  ShieldCheck,
  UserCheck,
  XCircle,
} from "lucide-react";
import { Button, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Status =
  "pending_approval" | "approved" | "executing" | "succeeded" | "failed" | "rejected" | "cancelled";
interface ActionProposal {
  id: string;
  action: "crm.lead.assign";
  level: 2;
  policy_version: string;
  target_type: "crm_lead";
  target_id: string;
  payload: { owner_user_id: string };
  payload_hash: string;
  reason: string;
  status: Status;
  proposed_by: string;
  proposer_role: string;
  reviewed_by: string | null;
  reviewer_role: string | null;
  review_note: string | null;
  reviewed_at: string | null;
  execution_attempts: number;
  result?: { status_code: number; body: unknown };
  failure_code: string | null;
  failure_detail: string | null;
  executed_at: string | null;
  created_at: string;
  updated_at: string;
}
interface AuditEntry {
  id: string;
  action_id: string;
  event: string;
  actor_id: string;
  actor_role: string;
  evidence: Record<string, unknown>;
  occurred_at: string;
}
interface ActionDetail {
  action: ActionProposal;
  audit: AuditEntry[];
}

function text(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}
async function reviewAction(data: FormData) {
  "use server";
  const id = text(data, "id");
  const decision = text(data, "decision");
  const client = await createServerClient();
  await client.post(`/api/v1/ai/actions/${id}/${decision}`, { note: text(data, "note") });
  revalidatePath("/admin/automation");
}
async function retryAction(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.post(`/api/v1/ai/actions/${text(data, "id")}/retry`, {});
  revalidatePath("/admin/automation");
}

const allowedStatuses = new Set<Status>([
  "pending_approval",
  "approved",
  "executing",
  "succeeded",
  "failed",
  "rejected",
  "cancelled",
]);

export default async function AutomationPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string }>;
}) {
  await requireAuth();
  const query = await searchParams;
  const selected = allowedStatuses.has(query.status as Status)
    ? (query.status as Status | undefined)
    : undefined;
  let actions: ActionProposal[] = [];
  let details = new Map<string, ActionDetail>();
  let error: string | null = null;
  try {
    const client = await createServerClient();
    const suffix = selected ? `?status=${selected}&limit=50` : "?limit=50";
    actions = (await client.get<{ data: ActionProposal[] }>(`/api/v1/ai/actions${suffix}`)).data;
    const loaded = await Promise.all(
      actions
        .slice(0, 20)
        .map((action) => client.get<ActionDetail>(`/api/v1/ai/actions/${action.id}`)),
    );
    details = new Map(loaded.map((detail) => [detail.action.id, detail]));
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load controlled actions";
  }
  const pending = actions.filter((action) => action.status === "pending_approval").length;
  const succeeded = actions.filter((action) => action.status === "succeeded").length;
  const failed = actions.filter((action) => action.status === "failed").length;
  return (
    <div className="space-y-7">
      <PageHeader
        icon={<ShieldCheck className="size-7" />}
        title="AI action control"
        description="A human checkpoint between an AI recommendation and a real system change. Every permitted action is bounded, independently reviewed, replay-safe, and permanently evidenced."
      />
      <Reveal>
        <section className="relative overflow-hidden rounded-3xl border border-border bg-foreground p-6 text-background shadow-xl sm:p-8">
          <div
            aria-hidden
            className="absolute -right-20 -top-24 size-72 rounded-full bg-primary/30 blur-3xl"
          />
          <div
            aria-hidden
            className="absolute bottom-0 left-1/3 h-px w-1/2 bg-gradient-to-r from-transparent via-secondary to-transparent"
          />
          <div className="relative grid gap-8 xl:grid-cols-[1fr_24rem]">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border border-background/15 bg-background/5 px-3 py-1.5 text-xs font-bold uppercase tracking-[0.18em]">
                <LockKeyhole className="size-3.5 text-secondary" />
                Policy 2026-07-19.v1
              </div>
              <h2 className="mt-5 max-w-3xl font-heading text-3xl font-black leading-tight sm:text-4xl">
                Automation earns permission one action at a time.
              </h2>
              <p className="mt-4 max-w-2xl text-sm leading-7 text-background/70">
                The production allowlist currently contains one reversible operation: assign one CRM
                lead to one owner. Bulk communication, publishing, advertising spend, fees, grades,
                security roles, and admission decisions cannot enter this queue.
              </p>
            </div>
            <div className="grid gap-3">
              <PolicyStep
                icon={<Bot />}
                label="AI proposes"
                detail="Exact target, reason and payload hash"
              />
              <PolicyStep
                icon={<UserCheck />}
                label="Human approves"
                detail="Independent reviewer with tool permission"
              />
              <PolicyStep
                icon={<Fingerprint />}
                label="System executes"
                detail="Immutable outcome and replay evidence"
              />
            </div>
          </div>
        </section>
      </Reveal>
      <section className="grid gap-4 sm:grid-cols-3">
        <Reveal delay={60}>
          <StatCard
            label="Awaiting human"
            value={pending}
            unit="proposals"
            tone={pending > 0 ? "warn" : "default"}
          />
        </Reveal>
        <Reveal delay={120}>
          <StatCard label="Completed safely" value={succeeded} unit="actions" tone="ok" />
        </Reveal>
        <Reveal delay={180}>
          <StatCard
            label="Needs retry review"
            value={failed}
            unit="actions"
            tone={failed > 0 ? "warn" : "default"}
          />
        </Reveal>
      </section>
      <Reveal delay={120}>
        <nav
          className="flex flex-wrap gap-2 rounded-2xl border border-border bg-surface p-3"
          aria-label="Action status"
        >
          <FilterLink label="All" />
          <FilterLink status="pending_approval" label="Awaiting approval" />
          <FilterLink status="succeeded" label="Succeeded" />
          <FilterLink status="failed" label="Failed" />
          <FilterLink status="rejected" label="Rejected" />
        </nav>
      </Reveal>
      {error ? (
        <EmptyState
          title="Action control is unavailable"
          description={error}
          icon={<AlertTriangle className="size-8" />}
        />
      ) : actions.length === 0 ? (
        <EmptyState
          title="No controlled actions in this view"
          description="When an authorised AI agent proposes an allowlisted low-risk action, its exact evidence and approval controls will appear here."
          icon={<ShieldCheck className="size-8" />}
        />
      ) : (
        <section className="space-y-5">
          {actions.map((action, index) => (
            <Reveal key={action.id} delay={Math.min(index * 45, 225)}>
              <ActionCard action={action} audit={details.get(action.id)?.audit ?? []} />
            </Reveal>
          ))}
        </section>
      )}
    </div>
  );
}

function PolicyStep({ icon, label, detail }: { icon: ReactNode; label: string; detail: string }) {
  return (
    <div className="group flex items-center gap-3 rounded-2xl border border-background/15 bg-background/5 p-3 transition duration-300 hover:-translate-y-0.5 hover:border-secondary/50 hover:bg-background/10">
      <span className="flex size-9 items-center justify-center rounded-xl bg-secondary/15 text-secondary [&>svg]:size-4">
        {icon}
      </span>
      <div>
        <p className="text-sm font-bold">{label}</p>
        <p className="text-xs text-background/60">{detail}</p>
      </div>
    </div>
  );
}
function FilterLink({ status, label }: { status?: Status; label: string }) {
  return (
    <a
      href={status ? `/admin/automation?status=${status}` : "/admin/automation"}
      className="rounded-full border border-border bg-background px-4 py-2 text-xs font-bold transition hover:-translate-y-0.5 hover:border-primary hover:text-primary"
    >
      {label}
    </a>
  );
}
function ActionCard({ action, audit }: { action: ActionProposal; audit: AuditEntry[] }) {
  const tone =
    action.status === "succeeded"
      ? "emerald"
      : action.status === "failed" || action.status === "rejected"
        ? "rose"
        : action.status === "pending_approval"
          ? "amber"
          : "slate";
  return (
    <article className="overflow-hidden rounded-3xl border border-border bg-surface shadow-sm transition duration-300 hover:shadow-md">
      <div className="grid xl:grid-cols-[1fr_22rem]">
        <div className="p-6">
          <div className="flex flex-wrap items-center gap-2">
            <StatusPill status={action.status} tone={tone} />
            <span className="rounded-full border border-border px-2.5 py-1 text-xs font-bold">
              Level {action.level} · low risk
            </span>
            <span className="text-xs text-muted-foreground">
              Attempt {action.execution_attempts}
            </span>
          </div>
          <h2 className="mt-4 font-heading text-2xl font-black">Assign one recruitment lead</h2>
          <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{action.reason}</p>
          <div className="mt-5 grid gap-3 sm:grid-cols-2">
            <Evidence label="Lead target" value={action.target_id} />
            <Evidence label="Proposed owner" value={action.payload.owner_user_id} />
            <Evidence
              label="Proposed by"
              value={`${action.proposed_by} · ${action.proposer_role}`}
            />
            <Evidence label="Payload fingerprint" value={action.payload_hash} />
          </div>
          {action.failure_detail ? (
            <div className="mt-4 flex gap-3 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-900">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <div>
                <strong>Execution was not hidden.</strong>
                <p className="mt-1">{action.failure_detail}</p>
              </div>
            </div>
          ) : null}
          {action.reviewed_by ? (
            <p className="mt-4 text-xs text-muted-foreground">
              Reviewed by <strong className="text-foreground">{action.reviewed_by}</strong>
              {action.review_note ? ` — ${action.review_note}` : ""}
            </p>
          ) : null}
          {action.status === "pending_approval" ? (
            <form
              action={reviewAction}
              className="mt-5 grid gap-2 border-t border-border pt-5 sm:grid-cols-[1fr_auto_auto]"
            >
              <input type="hidden" name="id" value={action.id} />
              <input
                required
                minLength={3}
                maxLength={1000}
                name="note"
                placeholder="What did you independently verify?"
                className="h-11 rounded-md border border-border bg-background px-3 text-sm"
              />
              <Button type="submit" name="decision" value="approve">
                <CheckCircle2 className="mr-2 size-4" />
                Approve and execute
              </Button>
              <Button type="submit" name="decision" value="reject" variant="secondary">
                <XCircle className="mr-2 size-4" />
                Reject
              </Button>
            </form>
          ) : action.status === "failed" ? (
            <form action={retryAction} className="mt-5">
              <input type="hidden" name="id" value={action.id} />
              <Button type="submit" variant="secondary">
                <RotateCcw className="mr-2 size-4" />
                Retry approved payload
              </Button>
            </form>
          ) : null}
        </div>
        <aside className="border-t border-border bg-muted/35 p-6 xl:border-l xl:border-t-0">
          <p className="text-xs font-extrabold uppercase tracking-[0.18em] text-primary">
            Immutable evidence trail
          </p>
          <div className="mt-5 space-y-0">
            {audit.length === 0 ? (
              <p className="text-sm text-muted-foreground">Evidence is loading.</p>
            ) : (
              audit.map((entry, index) => (
                <div key={entry.id} className="relative flex gap-3 pb-5 last:pb-0">
                  <div className="relative z-10 mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full border border-border bg-background text-primary">
                    <AuditIcon event={entry.event} />
                  </div>
                  {index < audit.length - 1 ? (
                    <span
                      aria-hidden
                      className="absolute left-[13px] top-7 h-[calc(100%-1rem)] w-px bg-border"
                    />
                  ) : null}
                  <div>
                    <p className="text-sm font-bold">{entry.event.replaceAll("_", " ")}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {entry.actor_id} · {new Date(entry.occurred_at).toLocaleString("en-GB")}
                    </p>
                  </div>
                </div>
              ))
            )}
          </div>
        </aside>
      </div>
    </article>
  );
}
function Evidence({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-xl bg-muted p-3">
      <p className="text-[0.65rem] font-extrabold uppercase tracking-[0.15em] text-muted-foreground">
        {label}
      </p>
      <p className="mt-1 truncate font-mono text-xs" title={value}>
        {value}
      </p>
    </div>
  );
}
function StatusPill({ status, tone }: { status: Status; tone: string }) {
  const classes =
    tone === "emerald"
      ? "bg-emerald-50 text-emerald-800"
      : tone === "rose"
        ? "bg-rose-50 text-rose-800"
        : tone === "amber"
          ? "bg-amber-50 text-amber-900"
          : "bg-slate-100 text-slate-800";
  return (
    <span className={`rounded-full px-2.5 py-1 text-xs font-extrabold ${classes}`}>
      {status.replaceAll("_", " ")}
    </span>
  );
}
function AuditIcon({ event }: { event: string }) {
  if (event.includes("succeeded") || event === "approved")
    return <CheckCircle2 className="size-3.5" />;
  if (event.includes("failed") || event === "rejected") return <XCircle className="size-3.5" />;
  return <ArrowRight className="size-3.5" />;
}
