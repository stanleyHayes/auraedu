import Link from "next/link";
import { ArrowLeft, SendHorizonal } from "lucide-react";
import { EmptyState, PageHeader } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import {
  channelLabel,
  deliveryDotStyle,
  deliveryStateStyle,
  effectiveDeliveryState,
  type Message,
} from "@/lib/admin-delivery";

interface HistoryEntry {
  status: string;
  at: string | null;
}

/** Status history is being added to the message payload; read it defensively from metadata. */
function statusHistory(message: Message): HistoryEntry[] {
  const raw = message.metadata?.status_history ?? message.metadata?.history;
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((entry) => {
    if (typeof entry !== "object" || entry === null) return [];
    const record = entry as Record<string, unknown>;
    const status = typeof record.status === "string" ? record.status : null;
    const at =
      typeof record.at === "string"
        ? record.at
        : typeof record.occurred_at === "string"
          ? record.occurred_at
          : null;
    return status ? [{ status, at }] : [];
  });
}

function formatTime(value: string | null | undefined) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function AdminDeliveryDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  await requireAuth();
  const { id } = await params;

  let message: Message | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClient();
    message = await client.get<Message>(`/api/v1/messages/${encodeURIComponent(id)}`);
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load the message";
  }

  if (error || !message) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<SendHorizonal className="size-7" />}
          title="Message detail"
          description="The message could not be loaded."
        />
        <EmptyState
          title="Could not load the message"
          description={error ?? "The message does not exist or is not visible to your account."}
          icon={<SendHorizonal className="size-8" />}
        />
        <Link
          href="/admin/delivery"
          className="inline-flex items-center gap-2 text-sm font-semibold text-primary underline-offset-4 hover:underline"
        >
          <ArrowLeft className="size-4" /> Back to delivery logs
        </Link>
      </div>
    );
  }

  const state = effectiveDeliveryState(message);
  const timeline: HistoryEntry[] = [
    { status: "created", at: message.created_at },
    ...(message.scheduled_at ? [{ status: "scheduled", at: message.scheduled_at }] : []),
    ...(message.sent_at ? [{ status: "sent", at: message.sent_at }] : []),
    ...(message.delivery_status
      ? [{ status: message.delivery_status, at: message.delivery_status_at ?? null }]
      : []),
    ...statusHistory(message),
  ];

  return (
    <div className="space-y-6">
      <Link
        href="/admin/delivery"
        className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground underline-offset-4 transition hover:text-primary hover:underline"
      >
        <ArrowLeft className="size-4" /> Delivery logs
      </Link>
      <PageHeader
        icon={<SendHorizonal className="size-7" />}
        title={message.subject || "Message detail"}
        description={`${channelLabel(message.channel)} · created ${formatTime(message.created_at)}`}
        action={
          <span
            className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold capitalize ${deliveryStateStyle(state)}`}
          >
            <span className={`size-2 rounded-full ${deliveryDotStyle(state)}`} aria-hidden="true" />
            {state.replaceAll("_", " ")}
          </span>
        }
      />

      {message.error ? (
        <div
          role="alert"
          className="rounded-xl border border-red-300 bg-red-50 p-4 text-sm text-red-900"
        >
          Delivery error: {message.error}
        </div>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-3">
        <section className="rounded-2xl border border-border bg-surface p-6 shadow-sm lg:col-span-2">
          <h2 className="font-heading text-xl font-bold">Content</h2>
          <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
                Recipient
              </dt>
              <dd className="mt-1 font-mono text-xs">{message.recipient_id}</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
                Template
              </dt>
              <dd className="mt-1 font-mono text-xs">{message.template_id ?? "Ad hoc"}</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
                Provider
              </dt>
              <dd className="mt-1 capitalize">{message.provider ?? "Not dispatched"}</dd>
            </div>
            <div>
              <dt className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
                Sent at
              </dt>
              <dd className="mt-1">{formatTime(message.sent_at)}</dd>
            </div>
          </dl>
          <p className="mt-5 whitespace-pre-wrap rounded-xl bg-background/70 p-4 text-sm leading-6 text-muted-foreground">
            {message.body}
          </p>
        </section>

        <section className="rounded-2xl border border-border bg-surface p-6 shadow-sm">
          <h2 className="font-heading text-xl font-bold">Status history</h2>
          {timeline.length === 0 ? (
            <p className="mt-4 text-sm text-muted-foreground">
              No status transitions have been recorded for this message yet.
            </p>
          ) : (
            <ol className="mt-4 space-y-3">
              {timeline.map((entry, index) => (
                <li key={`${entry.status}-${index}`} className="flex items-start gap-3">
                  <span
                    className={`mt-1.5 size-2.5 shrink-0 rounded-full ${deliveryDotStyle(entry.status)}`}
                    aria-hidden="true"
                  />
                  <div>
                    <p className="text-sm font-semibold capitalize">
                      {entry.status.replaceAll("_", " ")}
                    </p>
                    <p className="text-xs text-muted-foreground">{formatTime(entry.at)}</p>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </section>
      </div>
    </div>
  );
}
