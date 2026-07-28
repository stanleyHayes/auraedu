import Link from "next/link";
import { SendHorizonal } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import {
  channelLabel,
  deliveryDotStyle,
  deliveryStateStyle,
  effectiveDeliveryState,
  type Message,
} from "@/lib/admin-delivery";

type MessageList = OpenAPI.notification_v1.components["schemas"]["MessageList"];
type Template = OpenAPI.notification_v1.components["schemas"]["Template"];

const CHANNELS = ["email", "sms", "whatsapp", "in_app"];
const STATUSES = ["pending", "sent", "failed", "cancelled"];

interface DeliveryQuery {
  channel?: string;
  status?: string;
  cursor?: string;
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function AdminDeliveryPage({
  searchParams,
}: {
  searchParams: Promise<DeliveryQuery>;
}) {
  await requireAuth();
  const query = await searchParams;

  const params = new URLSearchParams();
  params.set("limit", "50");
  if (query.channel?.trim()) params.set("channel", query.channel.trim());
  if (query.status?.trim()) params.set("status", query.status.trim());
  if (query.cursor?.trim()) params.set("cursor", query.cursor.trim());

  let messages: Message[] = [];
  let templates: Template[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [messageResult, templateResult] = await Promise.allSettled([
      client.get<MessageList>(`/api/v1/messages?${params}`),
      client.get<OpenAPI.notification_v1.components["schemas"]["TemplateList"]>(
        "/api/v1/notification-templates?limit=100",
      ),
    ]);
    if (messageResult.status === "fulfilled") {
      messages = messageResult.value.data ?? [];
      nextCursor = messageResult.value.next_cursor ?? null;
    } else {
      error =
        messageResult.reason instanceof Error
          ? messageResult.reason.message
          : "Failed to load delivery logs";
    }
    templates = templateResult.status === "fulfilled" ? (templateResult.value.data ?? []) : [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load delivery logs";
  }

  const templateNames = new Map(templates.map((template) => [template.id, template.name]));
  const failing = messages.filter((message) =>
    ["failed", "bounced", "complained", "suppressed"].includes(effectiveDeliveryState(message)),
  ).length;
  const delivered = messages.filter((message) =>
    ["delivered", "sent", "accepted"].includes(effectiveDeliveryState(message)),
  ).length;

  const nextParams = new URLSearchParams();
  if (query.channel?.trim()) nextParams.set("channel", query.channel.trim());
  if (query.status?.trim()) nextParams.set("status", query.status.trim());
  if (nextCursor) nextParams.set("cursor", nextCursor);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<SendHorizonal className="size-7" />}
        title="Delivery logs"
        description="Every notification the school sends, with its delivery outcome. Failed deliveries are impossible to miss — investigate them before families ask."
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Messages on this page" value={messages.length} unit="records" />
        <StatCard label="Delivered or sent" value={delivered} tone="ok" />
        <StatCard label="Failed outcomes" value={failing} tone={failing > 0 ? "warn" : "default"} />
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
                {channelLabel(channel as Message["channel"])}
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
          {query.channel || query.status || query.cursor ? (
            <Link
              href="/admin/delivery"
              className="grid h-10 place-items-center rounded-md border border-border px-4 text-sm font-semibold text-muted-foreground transition hover:text-foreground"
            >
              Reset
            </Link>
          ) : null}
        </div>
      </form>

      {error ? (
        <EmptyState
          title="Could not load delivery logs"
          description={error}
          icon={<SendHorizonal className="size-8" />}
        />
      ) : (
        <>
          <DataTable
            caption="Notification delivery logs"
            rows={messages}
            keyExtractor={(message) => message.id}
            columns={[
              {
                key: "created_at",
                header: "Created at",
                cell: (message) => (
                  <span className="whitespace-nowrap text-sm">
                    {formatTime(message.created_at)}
                  </span>
                ),
              },
              {
                key: "channel",
                header: "Channel",
                cell: (message) => (
                  <span className="inline-flex rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-semibold capitalize">
                    {channelLabel(message.channel)}
                  </span>
                ),
              },
              {
                key: "recipient",
                header: "Recipient",
                cell: (message) => (
                  <span className="font-mono text-xs">{message.recipient_id.slice(0, 8)}…</span>
                ),
              },
              {
                key: "status",
                header: "Status",
                cell: (message) => {
                  const state = effectiveDeliveryState(message);
                  return (
                    <Link
                      href={`/admin/delivery/${message.id}`}
                      className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold capitalize ${deliveryStateStyle(state)}`}
                    >
                      <span
                        className={`size-2 rounded-full ${deliveryDotStyle(state)}`}
                        aria-hidden="true"
                      />
                      {state.replaceAll("_", " ")}
                    </Link>
                  );
                },
              },
              {
                key: "template",
                header: "Template",
                cell: (message) =>
                  message.template_id ? (
                    <span className="text-sm">
                      {templateNames.get(message.template_id) ?? (
                        <span className="font-mono text-xs">
                          {message.template_id.slice(0, 8)}…
                        </span>
                      )}
                    </span>
                  ) : (
                    <span className="text-muted-foreground">Ad hoc</span>
                  ),
              },
            ]}
            empty={
              <EmptyState
                title={
                  query.channel || query.status
                    ? "No messages match these filters"
                    : "No messages yet"
                }
                description={
                  query.channel || query.status
                    ? "Clear a filter to see more of the delivery trail."
                    : "Notifications sent by the school will appear here with their delivery outcomes."
                }
                icon={<SendHorizonal className="size-8" />}
              />
            }
          />
          {nextCursor ? (
            <div className="flex justify-end">
              <Link
                href={`/admin/delivery?${nextParams}`}
                className="rounded-md border border-border bg-surface px-4 py-2 text-sm font-semibold text-primary transition hover:-translate-y-0.5 hover:shadow-md"
              >
                Older messages →
              </Link>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
