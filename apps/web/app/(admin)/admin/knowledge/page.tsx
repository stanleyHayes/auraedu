import { revalidatePath } from "next/cache";
import { BookOpenCheck, CheckCircle2, Clock3, ShieldCheck } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Source = OpenAPI.knowledge_v1.components["schemas"]["Source"];

function text(formData: FormData, key: string) {
  const value = formData.get(key);
  return typeof value === "string" ? value.trim() : "";
}

async function createSource(formData: FormData) {
  "use server";
  const effective = new Date(text(formData, "effective_at"));
  const expiresRaw = text(formData, "expires_at");
  if (!Number.isFinite(effective.getTime())) return;
  const client = await createServerClient();
  await client.post("/api/v1/knowledge/sources", {
    source_type: text(formData, "source_type"),
    title: text(formData, "title"),
    owner: text(formData, "owner"),
    content: text(formData, "content"),
    confidentiality: text(formData, "confidentiality"),
    locale: text(formData, "locale"),
    effective_at: effective.toISOString(),
    expires_at: expiresRaw ? new Date(expiresRaw).toISOString() : null,
    programme: text(formData, "programme") || null,
    campus: null,
    intake: null,
  });
  revalidatePath("/admin/knowledge");
}

async function approveSource(formData: FormData) {
  "use server";
  const id = text(formData, "id");
  const reviewNote = text(formData, "review_note");
  if (!id || reviewNote.length < 3) return;
  const client = await createServerClient();
  await client.post(`/api/v1/knowledge/sources/${id}/approve`, { review_note: reviewNote });
  revalidatePath("/admin/knowledge");
}

async function retireSource(formData: FormData) {
  "use server";
  const id = text(formData, "id");
  if (!id) return;
  const client = await createServerClient();
  await client.post(`/api/v1/knowledge/sources/${id}/retire`, {});
  revalidatePath("/admin/knowledge");
}

export default async function KnowledgePage() {
  await requireAuth();
  let sources: Source[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    const response = await client.get<{ data: Source[] }>("/api/v1/knowledge/sources?limit=100");
    sources = response.data;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load knowledge sources";
  }
  const drafts = sources.filter((source) => source.status === "draft").length;
  const approved = sources.filter((source) => source.status === "approved").length;
  const expiring = sources.filter(
    (source) =>
      source.status === "approved" &&
      source.expires_at &&
      new Date(source.expires_at).getTime() < Date.now() + 30 * 86400000,
  ).length;
  const defaultEffective = new Date().toISOString().slice(0, 16);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BookOpenCheck className="size-7" />}
        title="Approved knowledge"
        description="Control exactly which institutional facts the public admissions assistant may retrieve and cite."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Approved sources" value={approved} />
        <StatCard label="Awaiting review" value={drafts} />
        <StatCard label="Expire within 30 days" value={expiring} />
      </div>

      <section className="rounded-xl border border-border bg-surface p-6">
        <div className="mb-5 flex items-start gap-3">
          <ShieldCheck className="mt-0.5 size-5 text-primary" />
          <div>
            <h2 className="font-heading text-xl font-bold">Add a controlled source</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              New content remains invisible to the assistant until a reviewer approves it.
            </p>
          </div>
        </div>
        <form action={createSource} className="grid gap-4 md:grid-cols-2">
          <label className="text-sm font-semibold">
            Title
            <input
              required
              minLength={3}
              maxLength={200}
              name="title"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              placeholder="2026 undergraduate fee schedule"
            />
          </label>
          <label className="text-sm font-semibold">
            Owner
            <input
              required
              minLength={2}
              maxLength={120}
              name="owner"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              placeholder="Admissions office"
            />
          </label>
          <label className="text-sm font-semibold">
            Source type
            <select
              name="source_type"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            >
              <option value="programme">Programme</option>
              <option value="admissions">Admissions</option>
              <option value="fees">Fees</option>
              <option value="scholarship">Scholarship</option>
              <option value="calendar">Calendar</option>
              <option value="policy">Policy</option>
              <option value="campus">Campus</option>
              <option value="accommodation">Accommodation</option>
              <option value="faq">FAQ</option>
              <option value="announcement">Announcement</option>
            </select>
          </label>
          <label className="text-sm font-semibold">
            Programme (optional)
            <input
              name="programme"
              maxLength={160}
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold">
            Effective from
            <input
              required
              type="datetime-local"
              name="effective_at"
              defaultValue={defaultEffective}
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold">
            Expires (optional)
            <input
              type="datetime-local"
              name="expires_at"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold">
            Visibility
            <select
              name="confidentiality"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            >
              <option value="public">Public assistant may retrieve after approval</option>
              <option value="internal">Internal only — never public retrieval</option>
            </select>
          </label>
          <label className="text-sm font-semibold">
            Language
            <select
              name="locale"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            >
              <option value="en-GH">English</option>
              <option value="fr-GH">Français</option>
            </select>
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            Approved source text
            <textarea
              required
              minLength={20}
              maxLength={100000}
              rows={8}
              name="content"
              className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal leading-6"
              placeholder="Paste the exact, verified institutional information…"
            />
          </label>
          <div className="md:col-span-2">
            <Button type="submit">Save draft for review</Button>
          </div>
        </form>
      </section>

      <section className="space-y-4">
        <h2 className="font-heading text-xl font-bold">Source register</h2>
        {error ? (
          <EmptyState
            title="Could not load knowledge"
            description={error}
            icon={<BookOpenCheck className="size-8" />}
          />
        ) : sources.length === 0 ? (
          <EmptyState
            title="No knowledge sources yet"
            description="Add the first verified programme, admissions, fee, or policy source above."
            icon={<BookOpenCheck className="size-8" />}
          />
        ) : (
          sources.map((source) => (
            <article key={source.id} className="rounded-xl border border-border bg-surface p-5">
              <div className="flex flex-col justify-between gap-4 md:flex-row">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-semibold">{source.title}</h3>
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold ${source.status === "approved" ? "bg-emerald-50 text-emerald-800" : source.status === "draft" ? "bg-amber-50 text-amber-800" : "bg-muted text-muted-foreground"}`}
                    >
                      {source.status}
                    </span>
                    <span className="rounded-full border border-border px-2.5 py-1 text-xs">
                      {source.confidentiality}
                    </span>
                    <span className="rounded-full border border-border px-2.5 py-1 text-xs">
                      {source.locale.startsWith("fr") ? "Français" : "English"}
                    </span>
                  </div>
                  <p className="mt-2 line-clamp-3 text-sm leading-6 text-muted-foreground">
                    {source.content}
                  </p>
                  <p className="mt-3 text-xs text-muted-foreground">
                    Owned by {source.owner} · effective{" "}
                    {new Date(source.effective_at).toLocaleDateString("en-GB")} · version{" "}
                    {source.version}
                  </p>
                </div>
                {source.status === "draft" ? (
                  <form action={approveSource} className="w-full shrink-0 space-y-2 md:w-72">
                    <input type="hidden" name="id" value={source.id} />
                    <label className="sr-only" htmlFor={`review-${source.id}`}>
                      Review note
                    </label>
                    <input
                      id={`review-${source.id}`}
                      required
                      minLength={3}
                      maxLength={500}
                      name="review_note"
                      placeholder="What was verified?"
                      className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                    />
                    <Button type="submit" className="w-full">
                      <CheckCircle2 className="mr-2 size-4" />
                      Approve source
                    </Button>
                  </form>
                ) : source.status === "approved" ? (
                  <form action={retireSource} className="shrink-0">
                    <input type="hidden" name="id" value={source.id} />
                    <Button type="submit" variant="secondary">
                      <Clock3 className="mr-2 size-4" />
                      Retire
                    </Button>
                  </form>
                ) : null}
              </div>
            </article>
          ))
        )}
      </section>
    </div>
  );
}
