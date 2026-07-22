import Link from "next/link";
import { PhoneCall, UserRoundSearch } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Lead = OpenAPI.crm_v1.components["schemas"]["Lead"];
type LeadList = OpenAPI.crm_v1.components["schemas"]["LeadList"];
type CallbackRequest = OpenAPI.crm_v1.components["schemas"]["CallbackRequest"];
interface CallbackList {
  data: CallbackRequest[];
}

const activeStages = new Set(["new", "contacted", "engaged", "qualified"]);

export default async function LeadsPage() {
  await requireAuth();
  let leads: Lead[] = [];
  let callbacks: CallbackRequest[] = [];
  let error: string | null = null;
  let callbackError: string | null = null;
  try {
    const client = await createServerClient();
    const [leadResult, callbackResult] = await Promise.allSettled([
      client.get<LeadList>("/api/v1/leads?limit=50"),
      client.get<CallbackList>("/api/v1/callback-requests?status=requested&limit=25"),
    ]);
    if (leadResult.status === "fulfilled") leads = leadResult.value.data;
    else
      error =
        leadResult.reason instanceof Error
          ? leadResult.reason.message
          : "Failed to load recruitment leads";
    if (callbackResult.status === "fulfilled") callbacks = callbackResult.value.data;
    else
      callbackError =
        callbackResult.reason instanceof Error
          ? callbackResult.reason.message
          : "Failed to load callback requests";
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load recruitment leads";
  }
  const unassigned = leads.filter((lead) => !lead.owner_user_id).length;
  const active = leads.filter((lead) => activeStages.has(lead.stage)).length;

  const leadByID = new Map(leads.map((lead) => [lead.id, lead]));

  return (
    <div className="space-y-8">
      <PageHeader
        icon={<UserRoundSearch className="size-7" />}
        title="Recruitment leads"
        description="Move every consented enquiry toward a clear next step without losing its source or history."
      />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Visible leads" value={leads.length} />
        <StatCard label="Active pipeline" value={active} />
        <StatCard label="Needs an owner" value={unassigned} />
        <StatCard label="Calls to confirm" value={callbacks.length} />
      </div>
      {error ? (
        <EmptyState
          title="Could not load leads"
          description={error}
          icon={<UserRoundSearch className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Recruitment leads"
          rows={leads}
          keyExtractor={(lead) => lead.id}
          columns={[
            {
              key: "name",
              header: "Prospect",
              cell: (lead) => (
                <div>
                  <Link
                    className="font-semibold text-primary underline-offset-4 hover:underline"
                    href={`/admin/leads/${lead.id}`}
                  >
                    {lead.first_name} {lead.last_name}
                  </Link>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {lead.email ?? lead.phone ?? "Contact protected"}
                  </p>
                </div>
              ),
            },
            {
              key: "stage",
              header: "Stage",
              cell: (lead) => (
                <span className="inline-flex rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-semibold capitalize">
                  {lead.stage.replaceAll("_", " ")}
                </span>
              ),
            },
            {
              key: "score",
              header: "Priority",
              cell: (lead) =>
                lead.score == null ? (
                  <span className="text-muted-foreground">Not scored</span>
                ) : (
                  <div>
                    <span className="font-heading text-lg font-black">{lead.score}</span>
                    <span className="ml-1 text-xs text-muted-foreground">/100</span>
                    <p className="text-[10px] uppercase tracking-wider text-muted-foreground">
                      {lead.score_confidence} confidence
                    </p>
                  </div>
                ),
            },
            {
              key: "source",
              header: "Source",
              cell: (lead) => (
                <span className="capitalize">{lead.source.replaceAll("_", " ")}</span>
              ),
            },
            {
              key: "owner",
              header: "Owner",
              cell: (lead) =>
                lead.owner_user_id ? (
                  <span className="font-mono text-xs">{lead.owner_user_id.slice(0, 8)}…</span>
                ) : (
                  <span className="text-amber-700">Unassigned</span>
                ),
            },
          ]}
          empty={
            <EmptyState
              title="No recruitment leads yet"
              description="Consented programme enquiries will appear here with their source and interaction history."
              icon={<UserRoundSearch className="size-8" />}
            />
          }
        />
      )}
      <section className="space-y-4" aria-labelledby="callback-queue-title">
        <div>
          <h2 id="callback-queue-title" className="text-xl font-bold">
            Callback requests
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Preferred times awaiting confirmation by the admissions team. A request is not yet a
            confirmed appointment.
          </p>
        </div>
        {callbackError ? (
          <EmptyState
            title="Could not load callback requests"
            description={callbackError}
            icon={<PhoneCall className="size-8" />}
          />
        ) : (
          <DataTable
            caption="Requested admissions callback times"
            rows={callbacks}
            keyExtractor={(callback) => callback.id}
            columns={[
              {
                key: "prospect",
                header: "Prospect",
                cell: (callback) => {
                  const lead = leadByID.get(callback.lead_id);
                  return lead ? (
                    <Link
                      className="font-semibold text-primary underline-offset-4 hover:underline"
                      href={`/admin/leads/${lead.id}`}
                    >
                      {lead.first_name} {lead.last_name}
                    </Link>
                  ) : (
                    <span className="font-mono text-xs">{callback.lead_id.slice(0, 8)}…</span>
                  );
                },
              },
              {
                key: "time",
                header: "Preferred time",
                cell: (callback) => (
                  <div>
                    <p className="font-semibold">
                      {new Intl.DateTimeFormat("en-GH", {
                        dateStyle: "medium",
                        timeStyle: "short",
                        timeZone: callback.timezone,
                      }).format(new Date(callback.preferred_at))}
                    </p>
                    <p className="mt-0.5 font-mono text-[10px] text-muted-foreground">
                      {callback.timezone}
                    </p>
                  </div>
                ),
              },
              {
                key: "language",
                header: "Language",
                cell: (callback) => <span className="uppercase">{callback.locale}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (callback) => (
                  <span className="inline-flex rounded-full border border-amber-300 bg-amber-50 px-2.5 py-1 text-xs font-semibold capitalize text-amber-900">
                    {callback.status}
                  </span>
                ),
              },
            ]}
            empty={
              <EmptyState
                title="No callback requests waiting"
                description="Preferred call times submitted through the website assistant will appear here for confirmation."
                icon={<PhoneCall className="size-8" />}
              />
            }
          />
        )}
      </section>
    </div>
  );
}
