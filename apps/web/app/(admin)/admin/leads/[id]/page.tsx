import { revalidatePath } from "next/cache";
import { notFound } from "next/navigation";
import {
  ArrowLeft,
  BrainCircuit,
  History,
  MinusCircle,
  PlusCircle,
  UserRoundSearch,
} from "lucide-react";
import { Button, EmptyState, PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Lead = OpenAPI.crm_v1.components["schemas"]["Lead"];
type Interaction = OpenAPI.crm_v1.components["schemas"]["Interaction"];
type InteractionList = OpenAPI.crm_v1.components["schemas"]["InteractionList"];
type Stage = OpenAPI.crm_v1.components["schemas"]["LeadStage"];
const stages: Stage[] = [
  "new",
  "contacted",
  "engaged",
  "qualified",
  "application_started",
  "application_completed",
  "under_review",
  "admitted",
  "offer_accepted",
  "deposit_paid",
  "enrolled",
  "lost",
  "deferred",
  "withdrawn",
];

function formString(formData: FormData, key: string) {
  const value = formData.get(key);
  return typeof value === "string" ? value : "";
}

async function updateStage(formData: FormData) {
  "use server";
  const id = formString(formData, "id");
  const stage = formString(formData, "stage") as Stage;
  if (!id || !stages.includes(stage)) return;
  const client = await createServerClient();
  await client.patch(`/api/v1/leads/${id}`, { stage });
  revalidatePath(`/admin/leads/${id}`);
  revalidatePath("/admin/leads");
}
async function addInteraction(formData: FormData) {
  "use server";
  const id = formString(formData, "id");
  const summary = formString(formData, "summary").trim();
  const channel = formString(formData, "channel") || "email";
  if (!id || !summary) return;
  const client = await createServerClient();
  await client.post(`/api/v1/leads/${id}/interactions`, {
    channel,
    direction: "outbound",
    summary,
  });
  revalidatePath(`/admin/leads/${id}`);
}
async function rescoreLead(formData: FormData) {
  "use server";
  const id = formString(formData, "id");
  if (!id) return;
  const client = await createServerClient();
  try {
    await client.post(`/api/v1/leads/${id}/score`, {});
  } catch {
    return;
  }
  revalidatePath(`/admin/leads/${id}`);
  revalidatePath("/admin/leads");
}

export default async function LeadPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAuth();
  const { id } = await params;
  const client = await createServerClient();
  let lead: Lead;
  let interactions: Interaction[];
  try {
    const result = await Promise.all([
      client.get<Lead>(`/api/v1/leads/${id}`),
      client.get<InteractionList>(`/api/v1/leads/${id}/interactions?limit=50`),
    ]);
    lead = result[0];
    interactions = result[1].data;
  } catch {
    return notFound();
  }
  return (
    <div className="space-y-6">
      <a
        href="/admin/leads"
        className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        All leads
      </a>
      <PageHeader
        icon={<UserRoundSearch className="size-7" />}
        title={`${lead.first_name} ${lead.last_name}`}
        description={`${lead.source.replaceAll("_", " ")} enquiry · captured ${new Date(lead.created_at).toLocaleDateString("en-GB")}`}
      />
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <section className="rounded-xl border border-border bg-surface p-6">
          <h2 className="font-heading text-xl font-bold">Interaction timeline</h2>
          <div className="mt-5 space-y-4">
            {interactions.length ? (
              interactions.map((item) => (
                <article key={item.id} className="border-l-2 border-primary/30 pl-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="text-sm font-semibold capitalize">
                      {item.channel} · {item.direction}
                    </p>
                    <time className="text-xs text-muted-foreground">
                      {new Date(item.occurred_at).toLocaleString("en-GB")}
                    </time>
                  </div>
                  <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                    {item.summary}
                  </p>
                </article>
              ))
            ) : (
              <EmptyState
                title="No interactions yet"
                description="Log the first follow-up so the next officer has the full context."
                icon={<History className="size-7" />}
              />
            )}
          </div>
        </section>
        <aside className="space-y-5">
          <section className="overflow-hidden rounded-xl border border-border bg-surface">
            <div className="bg-gradient-to-br from-primary/15 to-secondary/10 p-5">
              <div className="flex items-center justify-between">
                <BrainCircuit className="size-6 text-primary" />
                <span className="rounded-full bg-background/80 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider">
                  {lead.score_confidence ?? "not scored"} confidence
                </span>
              </div>
              <p className="mt-5 text-xs font-bold uppercase tracking-[0.18em] text-muted-foreground">
                Follow-up priority
              </p>
              <p className="mt-1 font-heading text-5xl font-black">
                {lead.score ?? "—"}
                <span className="text-base text-muted-foreground">/100</span>
              </p>
              <p className="mt-2 text-xs text-muted-foreground">
                Rules {lead.score_version ?? "not evaluated"}
              </p>
            </div>
            <div className="space-y-4 p-5">
              {lead.score_positive_factors?.slice(0, 3).map((factor) => (
                <div key={factor.code} className="flex gap-2 text-xs leading-5">
                  <PlusCircle className="mt-0.5 size-4 shrink-0 text-emerald-600" />
                  <span>
                    {factor.explanation}{" "}
                    <strong className="text-emerald-700">+{factor.points}</strong>
                  </span>
                </div>
              ))}
              {lead.score_negative_factors?.slice(0, 3).map((factor) => (
                <div key={factor.code} className="flex gap-2 text-xs leading-5">
                  <MinusCircle className="mt-0.5 size-4 shrink-0 text-amber-600" />
                  <span>
                    {factor.explanation} <strong className="text-amber-700">{factor.points}</strong>
                  </span>
                </div>
              ))}
              <p className="rounded-lg bg-muted p-3 text-[11px] leading-5 text-muted-foreground">
                This score prioritises follow-up. It does not decide admission and uses no protected
                personal characteristics.
              </p>
              <form action={rescoreLead}>
                <input type="hidden" name="id" value={lead.id} />
                <Button type="submit" variant="secondary" className="w-full">
                  Refresh explanation
                </Button>
              </form>
            </div>
          </section>
          <form action={updateStage} className="rounded-xl border border-border bg-surface p-5">
            <input type="hidden" name="id" value={lead.id} />
            <label htmlFor="stage" className="text-sm font-semibold">
              Pipeline stage
            </label>
            <select
              id="stage"
              name="stage"
              defaultValue={lead.stage}
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 text-sm"
            >
              {stages.map((stage) => (
                <option key={stage} value={stage}>
                  {stage.replaceAll("_", " ")}
                </option>
              ))}
            </select>
            <Button type="submit" className="mt-3 w-full">
              Update stage
            </Button>
          </form>
          <form action={addInteraction} className="rounded-xl border border-border bg-surface p-5">
            <h2 className="text-sm font-semibold">Log follow-up</h2>
            <label htmlFor="channel" className="sr-only">
              Channel
            </label>
            <select
              id="channel"
              name="channel"
              className="mt-3 h-11 w-full rounded-md border border-border bg-background px-3 text-sm"
            >
              <option value="email">Email</option>
              <option value="phone">Phone</option>
              <option value="whatsapp">WhatsApp</option>
              <option value="in_person">In person</option>
            </select>
            <label htmlFor="summary" className="sr-only">
              Interaction summary
            </label>
            <textarea
              id="summary"
              name="summary"
              required
              rows={4}
              placeholder="What happened and what is the next step?"
              className="mt-3 w-full rounded-md border border-border bg-background p-3 text-sm"
            />
            <Button type="submit" className="mt-3 w-full">
              Add to timeline
            </Button>
          </form>
        </aside>
      </div>
    </div>
  );
}
