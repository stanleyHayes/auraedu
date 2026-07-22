import { revalidatePath } from "next/cache";
import { ClipboardCheck, FileCheck2, GraduationCap, Send } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
type Application = OpenAPI.admissions_v1.components["schemas"]["Application"];
function text(data: FormData, key: string) {
  const value = data.get(key);
  return typeof value === "string" ? value.trim() : "";
}
async function review(data: FormData) {
  "use server";
  const id = text(data, "id"),
    decision = text(data, "decision"),
    note = text(data, "note");
  if (!id || !note) return;
  const client = await createServerClient();
  await client.post(`/api/v1/applications/${id}/review`, { decision, note });
  revalidatePath("/admin/admissions");
}
async function issueOffer(data: FormData) {
  "use server";
  const id = text(data, "id"),
    conditions = text(data, "conditions"),
    expires = new Date(text(data, "expires_at"));
  if (!id || !conditions || !Number.isFinite(expires.getTime())) return;
  const client = await createServerClient();
  await client.post(`/api/v1/applications/${id}/offer`, {
    conditions,
    expires_at: expires.toISOString(),
  });
  revalidatePath("/admin/admissions");
}
export default async function AdmissionsPage() {
  await requireAuth();
  let applications: Application[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    applications = (await client.get<{ data: Application[] }>("/api/v1/applications?limit=100"))
      .data;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load applications";
  }
  const submitted = applications.filter((item) => item.status === "submitted").length,
    admitted = applications.filter((item) => item.status === "admitted").length,
    accepted = applications.filter((item) => item.offer_status === "accepted").length;
  const defaultExpiry = new Date(Date.now() + 14 * 86400000).toISOString().slice(0, 16);
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<GraduationCap className="size-7" />}
        title="Admissions pipeline"
        description="Review complete applications, record accountable human decisions, and issue time-bound offers."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Awaiting review" value={submitted} />
        <StatCard label="Admitted" value={admitted} />
        <StatCard label="Offers accepted" value={accepted} />
      </div>
      <section className="space-y-4">
        <div>
          <h2 className="font-heading text-xl font-bold">Application register</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Applicant files stay tenant-scoped. AI and service accounts cannot decide admission or
            issue offers.
          </p>
        </div>
        {error ? (
          <EmptyState
            title="Could not load applications"
            description={error}
            icon={<ClipboardCheck className="size-8" />}
          />
        ) : applications.length === 0 ? (
          <EmptyState
            title="No applications yet"
            description="Started applications will appear here while applicants complete their checklist."
            icon={<ClipboardCheck className="size-8" />}
          />
        ) : (
          applications.map((item) => (
            <article key={item.id} className="rounded-xl border border-border bg-surface p-5">
              <div className="flex flex-col justify-between gap-5 lg:flex-row">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-semibold">
                      {(item.legal_name?.length ?? 0) > 0
                        ? item.legal_name
                        : "Application in progress"}
                    </h3>
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold ${item.status === "admitted" ? "bg-emerald-50 text-emerald-800" : item.status === "submitted" ? "bg-amber-50 text-amber-800" : "bg-muted text-muted-foreground"}`}
                    >
                      {item.status}
                    </span>
                    <span className="rounded-full border border-border px-2.5 py-1 text-xs">
                      {item.completion_percentage}% complete
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">
                    Applicant {item.applicant_user_id} · {item.programme_name} · {item.intake_name}
                  </p>
                  <p className="mt-2 text-xs text-muted-foreground">
                    <FileCheck2 className="mr-1 inline size-3.5" />
                    {item.documents.length} document reference
                    {item.documents.length === 1 ? "" : "s"} · offer {item.offer_status}
                  </p>
                  {item.review_note ? (
                    <p className="mt-3 rounded-md bg-muted p-3 text-sm">
                      Review: {item.review_note}
                    </p>
                  ) : null}
                </div>
                <div className="w-full shrink-0 lg:w-80">
                  {item.status === "submitted" ? (
                    <form action={review} className="space-y-2">
                      <input type="hidden" name="id" value={item.id} />
                      <label className="text-xs font-semibold">
                        Decision
                        <select
                          name="decision"
                          className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                        >
                          <option value="admitted">Admit</option>
                          <option value="rejected">Reject</option>
                        </select>
                      </label>
                      <label className="sr-only" htmlFor={`note-${item.id}`}>
                        Review evidence
                      </label>
                      <textarea
                        required
                        minLength={3}
                        maxLength={2000}
                        id={`note-${item.id}`}
                        name="note"
                        rows={3}
                        placeholder="Record the evidence checked and rationale"
                        className="w-full rounded-md border border-border bg-background p-3 text-sm"
                      />
                      <Button type="submit" className="w-full">
                        <ClipboardCheck className="mr-2 size-4" />
                        Record human decision
                      </Button>
                    </form>
                  ) : item.status === "admitted" && item.offer_status === "none" ? (
                    <form action={issueOffer} className="space-y-2">
                      <input type="hidden" name="id" value={item.id} />
                      <label className="sr-only" htmlFor={`conditions-${item.id}`}>
                        Offer conditions
                      </label>
                      <textarea
                        required
                        minLength={3}
                        maxLength={5000}
                        id={`conditions-${item.id}`}
                        name="conditions"
                        rows={3}
                        placeholder="Offer conditions and next steps"
                        className="w-full rounded-md border border-border bg-background p-3 text-sm"
                      />
                      <label className="text-xs font-semibold">
                        Accept by
                        <input
                          required
                          type="datetime-local"
                          name="expires_at"
                          defaultValue={defaultExpiry}
                          className="mt-1 h-10 w-full rounded-md border border-border bg-background px-3 font-normal"
                        />
                      </label>
                      <Button type="submit" className="w-full">
                        <Send className="mr-2 size-4" />
                        Issue time-bound offer
                      </Button>
                    </form>
                  ) : null}
                </div>
              </div>
            </article>
          ))
        )}
      </section>
    </div>
  );
}
