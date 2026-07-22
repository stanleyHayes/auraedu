import { revalidatePath } from "next/cache";
import { BadgeCheck, CalendarClock, Megaphone, PauseCircle, Send } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Campaign = OpenAPI.campaign_v1.components["schemas"]["Campaign"];

function value(formData: FormData, key: string) {
  const raw = formData.get(key);
  return typeof raw === "string" ? raw.trim() : "";
}

async function createCampaign(formData: FormData) {
  "use server";
  const startAt = new Date(value(formData, "start_at"));
  const endAt = new Date(value(formData, "end_at"));
  if (!Number.isFinite(startAt.getTime()) || !Number.isFinite(endAt.getTime())) return;
  const client = await createServerClient();
  await client.post("/api/v1/campaigns", {
    name: value(formData, "name"),
    objective: value(formData, "objective"),
    channel: value(formData, "channel"),
    audience_definition: value(formData, "audience_definition"),
    programme_ids: [],
    budget: Number(value(formData, "budget")),
    currency: value(formData, "currency").toUpperCase(),
    start_at: startAt.toISOString(),
    end_at: endAt.toISOString(),
  });
  revalidatePath("/admin/campaigns");
}

async function transition(formData: FormData) {
  "use server";
  const id = value(formData, "id");
  const action = value(formData, "action");
  if (!id || !["submit-for-approval", "approve", "publish", "pause"].includes(action)) return;
  const client = await createServerClient();
  await client.post(
    `/api/v1/campaigns/${id}/${action}`,
    action === "approve" ? { review_note: value(formData, "review_note") } : {},
  );
  revalidatePath("/admin/campaigns");
}

function localInput(date: Date) {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export default async function CampaignsPage() {
  await requireAuth();
  let campaigns: Campaign[] = [];
  let error: string | null = null;
  try {
    const client = await createServerClient();
    campaigns = (await client.get<{ data: Campaign[] }>("/api/v1/campaigns?limit=100")).data;
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "Failed to load campaigns";
  }
  const now = new Date();
  const pending = campaigns.filter((campaign) => campaign.status === "pending_approval").length;
  const live = campaigns.filter(
    (campaign) => campaign.status === "active" || campaign.status === "scheduled",
  ).length;
  const committed = campaigns
    .filter((campaign) => campaign.status !== "draft")
    .reduce((sum, campaign) => sum + campaign.budget, 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Megaphone className="size-7" />}
        title="Campaign control"
        description="Plan recruitment activity, preserve attribution, and require an independent review before anything goes live."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Live or scheduled" value={live} />
        <StatCard label="Awaiting review" value={pending} />
        <StatCard
          label="Committed budget"
          value={`GHS ${committed.toLocaleString("en-GH", { maximumFractionDigits: 2 })}`}
        />
      </div>

      <section className="rounded-xl border border-border bg-surface p-6">
        <div className="mb-5 flex items-start gap-3">
          <CalendarClock className="mt-0.5 size-5 text-primary" />
          <div>
            <h2 className="font-heading text-xl font-bold">Plan a campaign</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Every draft receives stable UTM tracking. Paid activity also requires explicit budget
              approval.
            </p>
          </div>
        </div>
        <form action={createCampaign} className="grid gap-4 md:grid-cols-2">
          <label className="text-sm font-semibold">
            Campaign name
            <input
              required
              minLength={3}
              maxLength={160}
              name="name"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              placeholder="August open day"
            />
          </label>
          <label className="text-sm font-semibold">
            Channel
            <select
              name="channel"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            >
              <option value="website">Website</option>
              <option value="email">Email</option>
              <option value="sms">SMS</option>
              <option value="whatsapp">WhatsApp</option>
              <option value="facebook">Facebook</option>
              <option value="instagram">Instagram</option>
              <option value="tiktok">TikTok</option>
              <option value="radio">Radio</option>
              <option value="event">Event</option>
              <option value="school_visit">School visit</option>
              <option value="referral">Referral</option>
            </select>
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            Objective
            <input
              required
              minLength={3}
              maxLength={500}
              name="objective"
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
              placeholder="Generate qualified applications for the next intake"
            />
          </label>
          <label className="text-sm font-semibold md:col-span-2">
            Audience definition
            <textarea
              required
              minLength={3}
              maxLength={2000}
              rows={3}
              name="audience_definition"
              className="mt-2 w-full rounded-md border border-border bg-background p-3 font-normal"
              placeholder="Prospective students and guardians interested in…"
            />
          </label>
          <label className="text-sm font-semibold">
            Budget
            <div className="mt-2 flex">
              <input
                name="currency"
                defaultValue="GHS"
                pattern="[A-Za-z]{3}"
                aria-label="Currency"
                className="h-11 w-20 rounded-l-md border border-border bg-muted px-3 font-normal uppercase"
              />
              <input
                required
                type="number"
                min="0"
                max="100000000"
                step="0.01"
                defaultValue="0"
                name="budget"
                className="h-11 min-w-0 flex-1 rounded-r-md border border-l-0 border-border bg-background px-3 font-normal"
              />
            </div>
          </label>
          <div />
          <label className="text-sm font-semibold">
            Starts
            <input
              required
              type="datetime-local"
              name="start_at"
              defaultValue={localInput(new Date(now.getTime() + 86_400_000))}
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <label className="text-sm font-semibold">
            Ends
            <input
              required
              type="datetime-local"
              name="end_at"
              defaultValue={localInput(new Date(now.getTime() + 8 * 86_400_000))}
              className="mt-2 h-11 w-full rounded-md border border-border bg-background px-3 font-normal"
            />
          </label>
          <div className="md:col-span-2">
            <Button type="submit">Save campaign draft</Button>
          </div>
        </form>
      </section>

      <section className="space-y-4">
        <div>
          <h2 className="font-heading text-xl font-bold">Campaign register</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            The person who submits a campaign cannot approve it. Ask another authorised reviewer to
            complete the approval.
          </p>
        </div>
        {error ? (
          <EmptyState
            title="Could not load campaigns"
            description={error}
            icon={<Megaphone className="size-8" />}
          />
        ) : campaigns.length === 0 ? (
          <EmptyState
            title="No campaigns yet"
            description="Create a draft to establish the approval and attribution trail."
            icon={<Megaphone className="size-8" />}
          />
        ) : (
          campaigns.map((campaign) => (
            <article key={campaign.id} className="rounded-xl border border-border bg-surface p-5">
              <div className="flex flex-col justify-between gap-5 lg:flex-row">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="font-semibold">{campaign.name}</h3>
                    <span
                      className={`rounded-full px-2.5 py-1 text-xs font-semibold ${campaign.status === "active" ? "bg-emerald-50 text-emerald-800" : campaign.status === "pending_approval" ? "bg-amber-50 text-amber-800" : "bg-muted text-muted-foreground"}`}
                    >
                      {campaign.status.replaceAll("_", " ")}
                    </span>
                    <span className="rounded-full border border-border px-2.5 py-1 text-xs">
                      {campaign.channel}
                    </span>
                  </div>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    {campaign.objective}
                  </p>
                  <dl className="mt-3 grid gap-1 text-xs text-muted-foreground sm:grid-cols-2">
                    <div>
                      Budget: {campaign.currency} {campaign.budget.toLocaleString()}
                    </div>
                    <div>Starts: {new Date(campaign.start_at).toLocaleString("en-GB")}</div>
                    <div className="truncate sm:col-span-2">
                      Tracking: {campaign.tracking_url_parameters}
                    </div>
                  </dl>
                </div>
                <div className="flex w-full shrink-0 flex-col gap-2 lg:w-72">
                  {campaign.status === "draft" ? (
                    <form action={transition}>
                      <input type="hidden" name="id" value={campaign.id} />
                      <input type="hidden" name="action" value="submit-for-approval" />
                      <Button type="submit" className="w-full">
                        <Send className="mr-2 size-4" />
                        Submit for approval
                      </Button>
                    </form>
                  ) : null}
                  {campaign.status === "pending_approval" ? (
                    <form action={transition} className="space-y-2">
                      <input type="hidden" name="id" value={campaign.id} />
                      <input type="hidden" name="action" value="approve" />
                      <label className="sr-only" htmlFor={`review-${campaign.id}`}>
                        Independent review note
                      </label>
                      <input
                        required
                        minLength={3}
                        maxLength={500}
                        id={`review-${campaign.id}`}
                        name="review_note"
                        placeholder="What did you verify?"
                        className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm"
                      />
                      <Button type="submit" className="w-full">
                        <BadgeCheck className="mr-2 size-4" />
                        Approve independently
                      </Button>
                    </form>
                  ) : null}
                  {campaign.status === "approved" ? (
                    <form action={transition}>
                      <input type="hidden" name="id" value={campaign.id} />
                      <input type="hidden" name="action" value="publish" />
                      <Button type="submit" className="w-full">
                        <Send className="mr-2 size-4" />
                        Schedule or publish
                      </Button>
                    </form>
                  ) : null}
                  {campaign.status === "active" || campaign.status === "scheduled" ? (
                    <form action={transition}>
                      <input type="hidden" name="id" value={campaign.id} />
                      <input type="hidden" name="action" value="pause" />
                      <Button type="submit" variant="secondary" className="w-full">
                        <PauseCircle className="mr-2 size-4" />
                        Pause campaign
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
