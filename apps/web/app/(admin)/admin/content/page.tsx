import { randomUUID } from "node:crypto";
import Link from "next/link";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import {
  BadgeCheck,
  Bot,
  Clock3,
  FileStack,
  MessagesSquare,
  PencilLine,
  Send,
  ShieldCheck,
  ShieldX,
  Sparkles,
} from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { ApiError } from "@auraedu/api-client";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type ContentDraft = OpenAPI.content_v1.components["schemas"]["ContentDraft"];
type ContentVersion = OpenAPI.content_v1.components["schemas"]["ContentVersion"];
type BrandProfile = OpenAPI.content_v1.components["schemas"]["BrandProfile"];
type ContentType = OpenAPI.content_v1.components["schemas"]["ContentType"];

const contentTypes: { value: ContentType; label: string }[] = [
  { value: "social_post", label: "Social post" },
  { value: "caption", label: "Caption" },
  { value: "ad_copy", label: "Advertisement copy" },
  { value: "video_script", label: "Video script" },
  { value: "landing_page", label: "Landing page" },
  { value: "email", label: "Email" },
  { value: "sms", label: "SMS" },
  { value: "whatsapp_sequence", label: "WhatsApp sequence" },
  { value: "programme_description", label: "Programme description" },
  { value: "brochure", label: "Brochure" },
  { value: "prospectus", label: "Prospectus" },
  { value: "event_invitation", label: "Event invitation" },
  { value: "scholarship_announcement", label: "Scholarship announcement" },
  { value: "radio_script", label: "Radio script" },
  { value: "faq", label: "FAQ" },
  { value: "applicant_guide", label: "Applicant guide" },
];

const statusStyle: Record<ContentDraft["status"], string> = {
  draft: "bg-sky-50 text-sky-800",
  pending_review: "bg-amber-50 text-amber-900",
  approved: "bg-emerald-50 text-emerald-800",
  rejected: "bg-rose-50 text-rose-800",
  expired: "bg-slate-100 text-slate-700",
};

function text(data: FormData, key: string) {
  const raw = data.get(key);
  return typeof raw === "string" ? raw.trim() : "";
}

function lines(data: FormData, key: string) {
  return text(data, key)
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function facts(data: FormData) {
  return lines(data, "facts").map((line) => {
    const divider = line.indexOf(":");
    return divider > 0
      ? { label: line.slice(0, divider).trim(), value: line.slice(divider + 1).trim() }
      : { label: "Fact", value: line };
  });
}

function optionalISO(data: FormData, key: string) {
  const raw = text(data, key);
  if (!raw) return null;
  const date = new Date(raw);
  return Number.isFinite(date.getTime()) ? date.toISOString() : null;
}

async function saveBrandProfile(data: FormData) {
  "use server";
  const client = await createServerClient();
  await client.put("/api/v1/content/brand-profile", {
    tone_of_voice: text(data, "tone_of_voice"),
    approved_terms: lines(data, "approved_terms"),
    prohibited_claims: lines(data, "prohibited_claims"),
    required_disclaimers: lines(data, "required_disclaimers"),
    locale: text(data, "locale"),
    expected_version: Number(text(data, "expected_version")),
  });
  revalidatePath("/admin/content");
}

async function generateDraft(data: FormData) {
  "use server";
  const client = await createServerClient();
  const campaignID = text(data, "campaign_id");
  const draft = await client.post<ContentDraft>(
    "/api/v1/content/generate",
    {
      content_type: text(data, "content_type"),
      title: text(data, "title"),
      brief: text(data, "brief"),
      audience: text(data, "audience"),
      locale: text(data, "locale"),
      campaign_id: campaignID || null,
      key_messages: lines(data, "key_messages"),
      facts: facts(data),
      expires_at: optionalISO(data, "expires_at"),
    },
    { headers: { "Idempotency-Key": `content-${randomUUID()}` } },
  );
  revalidatePath("/admin/content");
  redirect(`/admin/content?content=${draft.id}`);
}

async function reviseDraft(data: FormData) {
  "use server";
  const id = text(data, "id");
  if (!id) return;
  const client = await createServerClient();
  await client.patch(`/api/v1/content/${id}`, {
    content: text(data, "content"),
    change_note: text(data, "change_note"),
    expected_version: Number(text(data, "expected_version")),
    expires_at: optionalISO(data, "expires_at"),
  });
  revalidatePath("/admin/content");
  redirect(`/admin/content?content=${id}`);
}

async function reviewDraft(data: FormData) {
  "use server";
  const id = text(data, "id");
  const action = text(data, "action");
  if (!id || !["submit-for-review", "approve", "reject"].includes(action)) return;
  const client = await createServerClient();
  const body =
    action === "submit-for-review"
      ? { expected_version: Number(text(data, "expected_version")) }
      : {
          expected_version: Number(text(data, "expected_version")),
          review_note: text(data, "review_note"),
        };
  await client.post(`/api/v1/content/${id}/${action}`, body);
  revalidatePath("/admin/content");
  redirect(`/admin/content?content=${id}`);
}

function joined(values: string[]) {
  return values.join("\n");
}

function dateTimeLocal(value?: string | null) {
  if (!value) return "";
  const date = new Date(value);
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export default async function ContentStudioPage({
  searchParams,
}: {
  searchParams: Promise<{ content?: string }>;
}) {
  await requireAuth();
  const selectedID = (await searchParams).content?.trim() ?? "";
  let profile: BrandProfile | null = null;
  let drafts: ContentDraft[] = [];
  let selected: ContentDraft | null = null;
  let versions: ContentVersion[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    try {
      profile = await client.get<BrandProfile>("/api/v1/content/brand-profile");
    } catch (cause) {
      if (!(cause instanceof ApiError) || cause.status !== 404) throw cause;
    }
    drafts = (await client.get<{ data: ContentDraft[] }>("/api/v1/content?limit=100")).data;
    if (selectedID) {
      const detail = await client.get<{ content: ContentDraft; versions: ContentVersion[] }>(
        `/api/v1/content/${selectedID}`,
      );
      selected = detail.content;
      versions = detail.versions;
    }
  } catch (cause) {
    error =
      cause instanceof Error ? cause.message : "Failed to load the governed content workspace";
  }

  const pending = drafts.filter((draft) => draft.status === "pending_review").length;
  const approved = drafts.filter((draft) => draft.status === "approved").length;
  const blocked = drafts.filter((draft) => draft.compliance_status !== "pass").length;

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Growth · governed AI"
        icon={<Sparkles className="size-7" />}
        title="Content studio"
        description="Turn verified school facts into on-brand drafts, preserve every version, and require an independent human decision. AuraEDU never publishes generated content automatically."
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Awaiting review" value={pending} />
        <StatCard label="Approved versions" value={approved} />
        <StatCard label="Needs policy work" value={blocked} />
      </div>

      {error ? (
        <EmptyState
          icon={<ShieldX className="size-8" />}
          title="Content studio is unavailable"
          description={error}
        />
      ) : (
        <>
          <section className="relative overflow-hidden rounded-3xl border border-border bg-[var(--color-navy)] p-6 text-white shadow-xl sm:p-8">
            <span className="pointer-events-none absolute -right-16 -top-20 size-64 rounded-full bg-[var(--color-teal-bright)]/15 blur-3xl motion-safe:animate-[float-mark_8s_ease-in-out_infinite]" />
            <div className="relative grid gap-8 xl:grid-cols-[0.8fr_1.2fr]">
              <div>
                <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-xs font-bold uppercase tracking-[0.16em] text-[var(--color-signal)]">
                  <ShieldCheck className="size-4" /> Brand guardrail
                </div>
                <h2 className="mt-4 font-heading text-2xl font-extrabold">
                  Institutional voice, encoded
                </h2>
                <p className="mt-3 max-w-xl text-sm leading-6 text-white/70">
                  Generation stays inside this policy and the facts supplied in each brief.
                  Prohibited claims block review; missing disclaimers are surfaced before
                  submission.
                </p>
                <div className="mt-5 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
                    <div className="text-2xl font-black">v{profile?.version ?? 0}</div>
                    <div className="mt-1 text-white/60">Policy version</div>
                  </div>
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
                    <div className="text-2xl font-black">{profile?.locale ?? "—"}</div>
                    <div className="mt-1 text-white/60">Default locale</div>
                  </div>
                </div>
              </div>
              <form
                action={saveBrandProfile}
                className="grid gap-4 rounded-2xl border border-white/10 bg-white/95 p-5 text-[var(--foreground)] shadow-2xl md:grid-cols-2"
              >
                <input type="hidden" name="expected_version" value={profile?.version ?? 0} />
                <label className="text-sm font-semibold md:col-span-2">
                  Tone of voice
                  <textarea
                    required
                    minLength={3}
                    maxLength={1000}
                    rows={3}
                    name="tone_of_voice"
                    defaultValue={
                      profile?.tone_of_voice ?? "Warm, factual, inclusive and encouraging"
                    }
                    className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                  />
                </label>
                <PolicyList
                  name="approved_terms"
                  label="Approved terms"
                  value={joined(profile?.approved_terms ?? [])}
                  placeholder="learner-centred\nwhole-school community"
                />
                <PolicyList
                  name="prohibited_claims"
                  label="Prohibited claims"
                  value={joined(profile?.prohibited_claims ?? [])}
                  placeholder="guaranteed admission\nguaranteed results"
                />
                <label className="text-sm font-semibold md:col-span-2">
                  Required disclaimers{" "}
                  <span className="font-normal text-muted-foreground">(one per line)</span>
                  <textarea
                    rows={3}
                    name="required_disclaimers"
                    defaultValue={joined(profile?.required_disclaimers ?? [])}
                    className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                    placeholder="Terms and eligibility criteria apply"
                  />
                </label>
                <label className="text-sm font-semibold">
                  Locale
                  <input
                    required
                    pattern="[a-z]{2}(-[A-Z]{2})?"
                    name="locale"
                    defaultValue={profile?.locale ?? "en-GH"}
                    className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                  />
                </label>
                <div className="flex items-end">
                  <Button type="submit" className="w-full">
                    <ShieldCheck className="size-4" /> Save policy
                  </Button>
                </div>
              </form>
            </div>
          </section>

          <section className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
            <div className="rounded-3xl border border-border bg-surface p-6 shadow-sm">
              <div className="flex items-start gap-3">
                <span className="grid size-10 place-items-center rounded-xl bg-primary/10 text-primary">
                  <Bot className="size-5" />
                </span>
                <div>
                  <h2 className="font-heading text-xl font-bold">Generate from evidence</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Facts are required. The model is instructed not to invent fees, dates, outcomes,
                    rankings, or offers.
                  </p>
                </div>
              </div>
              {profile ? (
                <form action={generateDraft} className="mt-6 grid gap-4 md:grid-cols-2">
                  <label className="text-sm font-semibold">
                    Format
                    <select
                      name="content_type"
                      className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                    >
                      {contentTypes.map((item) => (
                        <option key={item.value} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="text-sm font-semibold">
                    Locale
                    <input
                      required
                      name="locale"
                      defaultValue={profile.locale}
                      className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                    />
                  </label>
                  <label className="text-sm font-semibold md:col-span-2">
                    Working title
                    <input
                      required
                      minLength={3}
                      maxLength={160}
                      name="title"
                      className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                      placeholder="August open day invitation"
                    />
                  </label>
                  <label className="text-sm font-semibold md:col-span-2">
                    Brief
                    <textarea
                      required
                      minLength={20}
                      maxLength={5000}
                      rows={4}
                      name="brief"
                      className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                      placeholder="Create a welcoming invitation that explains what families will experience…"
                    />
                  </label>
                  <label className="text-sm font-semibold md:col-span-2">
                    Audience
                    <textarea
                      required
                      minLength={3}
                      maxLength={1000}
                      rows={2}
                      name="audience"
                      className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                      placeholder="Prospective learners and guardians considering the next intake"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Key messages{" "}
                    <span className="font-normal text-muted-foreground">(one per line)</span>
                    <textarea
                      required
                      rows={4}
                      name="key_messages"
                      className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                      placeholder="Meet our teachers\nTour learning spaces"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Verified facts{" "}
                    <span className="font-normal text-muted-foreground">(Label: value)</span>
                    <textarea
                      required
                      rows={4}
                      name="facts"
                      className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
                      placeholder="Date: 30 August 2026\nVenue: Main campus"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Campaign ID{" "}
                    <span className="font-normal text-muted-foreground">(optional)</span>
                    <input
                      name="campaign_id"
                      className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                      placeholder="UUID"
                    />
                  </label>
                  <label className="text-sm font-semibold">
                    Expires <span className="font-normal text-muted-foreground">(optional)</span>
                    <input
                      type="datetime-local"
                      name="expires_at"
                      className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                    />
                  </label>
                  <div className="md:col-span-2">
                    <Button type="submit">
                      <Sparkles className="size-4" /> Generate governed draft
                    </Button>
                  </div>
                </form>
              ) : (
                <div className="mt-6 rounded-2xl border border-dashed border-border bg-muted/40 p-6 text-sm text-muted-foreground">
                  Save a brand policy first. AuraEDU will not generate ungoverned institutional
                  content.
                </div>
              )}
            </div>

            <div className="rounded-3xl border border-border bg-surface p-6 shadow-sm">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 className="font-heading text-xl font-bold">Draft register</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {drafts.length} governed {drafts.length === 1 ? "asset" : "assets"}
                  </p>
                </div>
                <FileStack className="size-6 text-primary" />
              </div>
              <div className="mt-5 max-h-[48rem] space-y-3 overflow-y-auto pr-1">
                {drafts.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
                    No content drafts yet.
                  </div>
                ) : (
                  drafts.map((draft, index) => (
                    <Link
                      key={draft.id}
                      href={`/admin/content?content=${draft.id}`}
                      className={`group block rounded-2xl border p-4 transition duration-200 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md motion-reduce:transform-none ${selectedID === draft.id ? "border-primary bg-primary/5" : "border-border bg-background"}`}
                      style={{ animationDelay: `${Math.min(index, 8) * 45}ms` }}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <h3 className="truncate font-bold">{draft.title}</h3>
                          <p className="mt-1 text-xs uppercase tracking-wide text-muted-foreground">
                            {draft.content_type.replaceAll("_", " ")} · v{draft.version}
                          </p>
                        </div>
                        <span
                          className={`shrink-0 rounded-full px-2.5 py-1 text-[11px] font-bold ${statusStyle[draft.status]}`}
                        >
                          {draft.status.replaceAll("_", " ")}
                        </span>
                      </div>
                      <div className="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
                        <ShieldCheck className="size-3.5" /> Compliance:{" "}
                        {draft.compliance_status.replaceAll("_", " ")}
                      </div>
                    </Link>
                  ))
                )}
              </div>
            </div>
          </section>

          {selected ? <DraftWorkspace draft={selected} versions={versions} /> : null}
        </>
      )}
    </div>
  );
}

function PolicyList({
  name,
  label,
  value,
  placeholder,
}: {
  name: string;
  label: string;
  value: string;
  placeholder: string;
}) {
  return (
    <label className="text-sm font-semibold">
      {label} <span className="font-normal text-muted-foreground">(one per line)</span>
      <textarea
        rows={4}
        name={name}
        defaultValue={value}
        className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal"
        placeholder={placeholder}
      />
    </label>
  );
}

function DraftWorkspace({ draft, versions }: { draft: ContentDraft; versions: ContentVersion[] }) {
  const canRevise = draft.status === "draft" || draft.status === "rejected";
  return (
    <section
      id="draft-workspace"
      className="overflow-hidden rounded-3xl border border-border bg-surface shadow-lg motion-safe:animate-[slide-up_260ms_var(--ease-out-quart)]"
    >
      <div className="border-b border-border bg-gradient-to-r from-primary/10 via-transparent to-[var(--color-signal)]/10 p-6 sm:p-8">
        <div className="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={`rounded-full px-3 py-1 text-xs font-bold ${statusStyle[draft.status]}`}
              >
                {draft.status.replaceAll("_", " ")}
              </span>
              <span className="rounded-full border border-border bg-background px-3 py-1 text-xs font-semibold">
                Version {draft.version}
              </span>
              <span className="rounded-full border border-border bg-background px-3 py-1 text-xs font-semibold">
                Policy v{draft.brand_profile_version}
              </span>
            </div>
            <h2 className="mt-4 font-heading text-2xl font-extrabold">{draft.title}</h2>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{draft.brief}</p>
          </div>
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Clock3 className="size-4" /> Updated{" "}
            {new Date(draft.updated_at).toLocaleString("en-GB")}
          </div>
        </div>
      </div>
      <div className="grid gap-0 xl:grid-cols-[1.25fr_0.75fr]">
        <div className="space-y-6 p-6 sm:p-8 xl:border-r xl:border-border">
          <div>
            <div className="mb-3 flex items-center gap-2">
              <MessagesSquare className="size-5 text-primary" />
              <h3 className="font-heading text-lg font-bold">Current copy</h3>
            </div>
            <div className="whitespace-pre-wrap rounded-2xl border border-border bg-background p-5 text-sm leading-7">
              {draft.content}
            </div>
          </div>
          {canRevise ? (
            <form
              action={reviseDraft}
              className="space-y-4 rounded-2xl border border-border bg-muted/30 p-5"
            >
              <input type="hidden" name="id" value={draft.id} />
              <input type="hidden" name="expected_version" value={draft.version} />
              <div className="flex items-center gap-2">
                <PencilLine className="size-4 text-primary" />
                <h3 className="font-bold">Create the next version</h3>
              </div>
              <label className="block text-sm font-semibold">
                Revised copy
                <textarea
                  required
                  maxLength={50000}
                  rows={9}
                  name="content"
                  defaultValue={draft.content}
                  className="mt-2 w-full rounded-xl border border-border bg-background p-3 font-normal leading-6"
                />
              </label>
              <div className="grid gap-4 md:grid-cols-2">
                <label className="text-sm font-semibold">
                  Change note
                  <input
                    required
                    minLength={3}
                    maxLength={500}
                    name="change_note"
                    className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                    placeholder="Corrected event date"
                  />
                </label>
                <label className="text-sm font-semibold">
                  New expiry <span className="font-normal text-muted-foreground">(optional)</span>
                  <input
                    type="datetime-local"
                    name="expires_at"
                    defaultValue={dateTimeLocal(draft.expires_at)}
                    className="mt-2 h-11 w-full rounded-xl border border-border bg-background px-3 font-normal"
                  />
                </label>
              </div>
              <Button type="submit" variant="secondary">
                <PencilLine className="size-4" /> Save as version {draft.version + 1}
              </Button>
            </form>
          ) : null}
        </div>
        <aside className="space-y-6 bg-muted/20 p-6 sm:p-8">
          <div>
            <div className="flex items-center gap-2">
              <ShieldCheck className="size-5 text-primary" />
              <h3 className="font-heading text-lg font-bold">Compliance gate</h3>
            </div>
            <div
              className={`mt-3 rounded-2xl border p-4 ${draft.compliance_status === "pass" ? "border-emerald-200 bg-emerald-50 text-emerald-900" : "border-amber-200 bg-amber-50 text-amber-950"}`}
            >
              <div className="font-bold capitalize">
                {draft.compliance_status.replaceAll("_", " ")}
              </div>
              {draft.compliance_findings.length === 0 ? (
                <p className="mt-1 text-sm">No policy findings on this version.</p>
              ) : (
                <ul className="mt-3 space-y-2 text-sm">
                  {draft.compliance_findings.map((finding, index) => (
                    <li key={`${finding.rule}-${index}`} className="flex gap-2">
                      <ShieldX className="mt-0.5 size-4 shrink-0" />
                      <span>{finding.message}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
          {draft.status === "draft" ? (
            <form action={reviewDraft}>
              <input type="hidden" name="id" value={draft.id} />
              <input type="hidden" name="action" value="submit-for-review" />
              <input type="hidden" name="expected_version" value={draft.version} />
              <Button
                type="submit"
                className="w-full"
                disabled={draft.compliance_status !== "pass"}
              >
                <Send className="size-4" /> Submit exact version for review
              </Button>
            </form>
          ) : null}
          {draft.status === "pending_review" ? (
            <div className="space-y-3 rounded-2xl border border-border bg-background p-4">
              <div>
                <h3 className="font-bold">Independent decision</h3>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  The submitting user cannot review their own draft.
                </p>
              </div>
              {["approve", "reject"].map((action) => (
                <form key={action} action={reviewDraft} className="space-y-2">
                  <input type="hidden" name="id" value={draft.id} />
                  <input type="hidden" name="action" value={action} />
                  <input type="hidden" name="expected_version" value={draft.version} />
                  <textarea
                    required
                    minLength={3}
                    maxLength={1000}
                    rows={2}
                    name="review_note"
                    aria-label={`${action} review note`}
                    className="w-full rounded-xl border border-border bg-background p-3 text-sm"
                    placeholder={
                      action === "approve"
                        ? "Facts and policy verified"
                        : "Explain the required correction"
                    }
                  />
                  <Button
                    type="submit"
                    variant={action === "approve" ? "primary" : "secondary"}
                    className="w-full"
                  >
                    {action === "approve" ? (
                      <BadgeCheck className="size-4" />
                    ) : (
                      <ShieldX className="size-4" />
                    )}
                    {action === "approve" ? "Approve this version" : "Reject with evidence"}
                  </Button>
                </form>
              ))}
            </div>
          ) : null}
          {draft.review_note ? (
            <div className="rounded-2xl border border-border bg-background p-4">
              <div className="text-xs font-bold uppercase tracking-wide text-muted-foreground">
                Review record
              </div>
              <p className="mt-2 text-sm leading-6">{draft.review_note}</p>
            </div>
          ) : null}
          <div>
            <h3 className="font-heading text-lg font-bold">Immutable history</h3>
            <div className="mt-3 space-y-3">
              {versions.map((version) => (
                <details
                  key={version.version}
                  className="group rounded-xl border border-border bg-background p-3"
                >
                  <summary className="cursor-pointer list-none text-sm font-bold">
                    Version {version.version} · {version.status.replaceAll("_", " ")}
                    <span className="ml-2 font-normal text-muted-foreground">
                      {version.change_note}
                    </span>
                  </summary>
                  <p className="mt-3 whitespace-pre-wrap border-t border-border pt-3 text-xs leading-6 text-muted-foreground">
                    {version.content}
                  </p>
                </details>
              ))}
            </div>
          </div>
          <div className="rounded-2xl border border-dashed border-border p-4 text-xs leading-5 text-muted-foreground">
            <strong className="text-foreground">No publish button by design.</strong> Approval
            records that a version is safe to use. Distribution remains an explicit channel workflow
            with its own permissions and evidence.
          </div>
        </aside>
      </div>
    </section>
  );
}
