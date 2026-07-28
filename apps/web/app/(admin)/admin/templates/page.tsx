import Link from "next/link";
import { MailCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { AdminTemplateRowActions } from "@/components/admin-template-row-actions";
import { AdminTemplateSheet } from "@/components/admin-template-sheet";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type Template = OpenAPI.notification_v1.components["schemas"]["Template"];
type TemplateList = OpenAPI.notification_v1.components["schemas"]["TemplateList"];
type Channel = OpenAPI.notification_v1.components["schemas"]["Channel"];
type TemplateStatus = OpenAPI.notification_v1.components["schemas"]["TemplateStatus"];

const CHANNELS: Channel[] = ["email", "sms", "whatsapp", "in_app", "push"];
const STATUSES: TemplateStatus[] = ["active", "archived"];

interface TemplatesQuery {
  channel?: string;
  status?: string;
}

function formatTime(value?: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function AdminTemplatesPage({
  searchParams,
}: {
  searchParams: Promise<TemplatesQuery>;
}) {
  await requireAuth();
  const query = await searchParams;

  const params = new URLSearchParams();
  params.set("limit", "100");
  if (query.channel?.trim()) params.set("channel", query.channel.trim());
  if (query.status?.trim()) params.set("status", query.status.trim());

  let templates: Template[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<TemplateList>(`/api/v1/notification-templates?${params}`);
    templates = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load notification templates";
  }

  const active = templates.filter((template) => template.status === "active").length;
  const archived = templates.length - active;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<MailCheck className="size-7" />}
        title="Message templates"
        description="Reusable email, SMS, WhatsApp, in-app and push copy. Active templates power journeys and direct sends — archive instead of deleting when a journey may still reference one."
        action={<AdminTemplateSheet mode="create" />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Templates listed" value={templates.length} unit="records" />
        <StatCard label="Active" value={active} tone="ok" />
        <StatCard label="Archived" value={archived} tone={archived > 0 ? "warn" : "default"} />
      </div>

      <form
        className="flex flex-col gap-3 rounded-2xl border border-border bg-surface p-4 sm:flex-row sm:items-end"
        method="get"
      >
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Channel
          <select
            name="channel"
            defaultValue={query.channel ?? ""}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          >
            <option value="">All channels</option>
            {CHANNELS.map((channel) => (
              <option key={channel} value={channel}>
                {channel.replaceAll("_", " ")}
              </option>
            ))}
          </select>
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Status
          <select
            name="status"
            defaultValue={query.status ?? ""}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          >
            <option value="">All statuses</option>
            {STATUSES.map((status) => (
              <option key={status} value={status} className="capitalize">
                {status}
              </option>
            ))}
          </select>
        </label>
        <div className="flex gap-2">
          <button
            type="submit"
            className="h-10 rounded-md bg-primary px-5 text-sm font-bold text-primary-foreground transition hover:-translate-y-0.5 hover:shadow-md"
          >
            Apply filters
          </button>
          {query.channel || query.status ? (
            <Link
              href="/admin/templates"
              className="grid h-10 place-items-center rounded-md border border-border px-4 text-sm font-semibold text-muted-foreground transition hover:text-foreground"
            >
              Reset
            </Link>
          ) : null}
        </div>
      </form>

      {error ? (
        <EmptyState
          title="Could not load templates"
          description={error}
          icon={<MailCheck className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Notification templates"
          rows={templates}
          keyExtractor={(template) => template.id}
          columns={[
            {
              key: "name",
              header: "Name",
              cell: (template) => (
                <div>
                  <div className="font-semibold">{template.name}</div>
                  {template.subject_template ? (
                    <div className="mt-0.5 max-w-md truncate text-xs text-muted-foreground">
                      {template.subject_template}
                    </div>
                  ) : null}
                </div>
              ),
            },
            {
              key: "channel",
              header: "Channel",
              cell: (template) => (
                <span className="inline-flex rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-semibold capitalize">
                  {template.channel.replaceAll("_", " ")}
                </span>
              ),
            },
            {
              key: "status",
              header: "Status",
              cell: (template) => {
                const style =
                  template.status === "active"
                    ? "bg-emerald-50 text-emerald-800"
                    : "bg-muted text-muted-foreground";
                return (
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${style}`}
                  >
                    {template.status}
                  </span>
                );
              },
            },
            {
              key: "updated",
              header: "Updated",
              cell: (template) => (
                <span className="text-sm text-muted-foreground">
                  {formatTime(template.updated_at)}
                </span>
              ),
            },
            {
              key: "actions",
              header: "Actions",
              cell: (template) => (
                <div className="flex items-center justify-end gap-1">
                  <AdminTemplateSheet mode="edit" initial={template} />
                  <AdminTemplateRowActions template={template} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              title="No templates yet"
              description="Create the first template above; journeys and direct sends can only use active templates."
              icon={<MailCheck className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
