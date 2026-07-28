import { Send } from "lucide-react";
import {
  PageHeader,
  DataTable,
  EmptyState,
  StatCard,
  Reveal,
  Button,
  Input,
  Label,
  Select,
} from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref, formatDateTime } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type Message = OpenAPI.notification_v1.components["schemas"]["Message"];

const CHANNELS = ["email", "sms", "whatsapp", "in_app", "push"] as const;
const STATUSES = ["pending", "sent", "failed", "cancelled"] as const;

interface TenantDeliveryPageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{
    channel?: string;
    status?: string;
    recipient_id?: string;
    cursor?: string;
  }>;
}

export default async function TenantDeliveryPage({
  params,
  searchParams,
}: TenantDeliveryPageProps) {
  const { code } = await params;
  const filters = await searchParams;
  await requireAuth();

  let messages: Message[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const res = await client.get<{ data?: Message[]; next_cursor?: string | null }>(
      `/api/v1/messages?${drilldownQuery(
        {
          channel: filters.channel,
          status: filters.status,
          recipient_id: filters.recipient_id,
        },
        filters.cursor,
      )}`,
    );
    messages = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load message delivery";
  }

  const sent = messages.filter((m) => m.status === "sent").length;
  const failed = messages.filter((m) => m.status === "failed").length;
  const nextHref = nextPageHref(`/superadmin/tenants/${code}/delivery`, filters, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<Send className="size-7" />}
          title="Notification delivery"
          description={`Read-only message delivery for ${code}.`}
        />
      </Reveal>

      <Reveal delay={40}>
        <form
          method="get"
          className="card grid gap-3 rounded-[var(--radius-md)] p-4 sm:grid-cols-2 lg:grid-cols-4"
        >
          <div className="space-y-1.5">
            <Label htmlFor="channel">Channel</Label>
            <Select id="channel" name="channel" defaultValue={filters.channel ?? ""}>
              <option value="">All channels</option>
              {CHANNELS.map((channel) => (
                <option key={channel} value={channel}>
                  {channel.replace("_", " ")}
                </option>
              ))}
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="status">Status</Label>
            <Select id="status" name="status" defaultValue={filters.status ?? ""}>
              <option value="">All statuses</option>
              {STATUSES.map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="recipient_id">Recipient ID</Label>
            <Input
              id="recipient_id"
              name="recipient_id"
              defaultValue={filters.recipient_id ?? ""}
              placeholder="Recipient UUID"
            />
          </div>
          <div className="flex items-end">
            <Button type="submit" variant="secondary" className="w-full">
              Apply filters
            </Button>
          </div>
        </form>
      </Reveal>

      {error ? (
        <EmptyState
          title="Delivery data unavailable"
          description={error}
          icon={<Send className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-3">
              <StatCard
                label="Messages"
                value={`${messages.length}${nextCursor ? "+" : ""}`}
                unit="this page"
              />
              <StatCard label="Sent" value={sent} unit="messages" tone="ok" />
              <StatCard
                label="Failed"
                value={failed}
                unit="messages"
                tone={failed > 0 ? "warn" : "default"}
              />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <DataTable
              caption={`Notification delivery for tenant ${code}`}
              rows={messages}
              keyExtractor={(m) => m.id}
              columns={[
                {
                  key: "created",
                  header: "Created",
                  cell: (m) => (
                    <span className="font-mono text-xs">{formatDateTime(m.created_at)}</span>
                  ),
                },
                {
                  key: "channel",
                  header: "Channel",
                  className: "w-28",
                  cell: (m) => (
                    <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize">
                      {m.channel.replace("_", " ")}
                    </span>
                  ),
                },
                {
                  key: "recipient",
                  header: "Recipient",
                  cell: (m) => (
                    <span className="font-mono text-xs" title={m.recipient_id}>
                      {m.recipient_id.slice(0, 8)}…
                    </span>
                  ),
                },
                {
                  key: "subject",
                  header: "Subject",
                  cell: (m) => m.subject || "—",
                },
                {
                  key: "status",
                  header: "Status",
                  className: "w-24",
                  cell: (m) =>
                    m.status === "sent" ? (
                      <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                        {m.status}
                      </span>
                    ) : m.status === "failed" ? (
                      <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                        {m.status}
                      </span>
                    ) : (
                      <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                        {m.status}
                      </span>
                    ),
                },
                {
                  key: "delivery",
                  header: "Delivery",
                  cell: (m) => m.delivery_status ?? "—",
                },
              ]}
              empty={
                <EmptyState
                  title="No messages"
                  description="No messages match these filters for this school."
                  icon={<Send className="size-8" />}
                />
              }
            />
          </Reveal>

          <SuperadminPagination nextHref={nextHref} />
        </>
      )}
    </div>
  );
}
