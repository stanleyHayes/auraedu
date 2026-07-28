import Link from "next/link";
import { ScrollText } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

type AuditLog = OpenAPI.audit_v1.components["schemas"]["AuditLog"];
/** The audit backend is gaining source_service/action columns; read them defensively until the contract lands. */
type AuditLogRow = AuditLog & { source_service?: string | null; action?: string | null };
interface AuditLogList {
  data?: AuditLogRow[];
  next_cursor?: string | null;
}

interface AuditQuery {
  event_type?: string;
  actor_id?: string;
  source_service?: string;
  from?: string;
  to?: string;
  cursor?: string;
}

const FILTER_KEYS = ["event_type", "actor_id", "source_service", "from", "to"] as const;

function rowField(log: AuditLogRow, key: "source_service" | "action"): string | null {
  const direct = log[key];
  if (typeof direct === "string" && direct.length > 0) return direct;
  const fromMetadata = log.metadata?.[key];
  return typeof fromMetadata === "string" && fromMetadata.length > 0 ? fromMetadata : null;
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat("en-GH", { dateStyle: "medium", timeStyle: "short" }).format(
    new Date(value),
  );
}

export default async function AdminAuditPage({
  searchParams,
}: {
  searchParams: Promise<AuditQuery>;
}) {
  await requireAuth();
  const query = await searchParams;

  const params = new URLSearchParams();
  params.set("limit", "50");
  for (const key of FILTER_KEYS) {
    const value = query[key]?.trim();
    if (value) params.set(key, value);
  }
  if (query.cursor?.trim()) params.set("cursor", query.cursor.trim());

  let logs: AuditLogRow[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<AuditLogList>(`/api/v1/audit/logs?${params}`);
    logs = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load audit logs";
  }

  const filtersActive = FILTER_KEYS.some((key) => query[key]?.trim());
  const nextParams = new URLSearchParams();
  for (const key of FILTER_KEYS) {
    const value = query[key]?.trim();
    if (value) nextParams.set(key, value);
  }
  if (nextCursor) nextParams.set("cursor", nextCursor);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ScrollText className="size-7" />}
        title="Audit log"
        description="Every privileged action in this school, in order. Filters are applied by the audit service; results appear even while a filter is not yet honoured."
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Events on this page" value={logs.length} unit="records" />
        <StatCard label="Filters active" value={filtersActive ? "Yes" : "No"} />
        <StatCard label="More pages" value={nextCursor ? "Available" : "None"} unit="cursor" />
      </div>

      <form
        className="flex flex-col gap-3 rounded-2xl border border-border bg-surface p-4 lg:flex-row lg:items-end"
        method="get"
      >
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Event type
          <input
            type="text"
            name="event_type"
            defaultValue={query.event_type}
            placeholder="student.created"
            className="mt-2 block h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground lg:w-44"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Actor ID
          <input
            type="text"
            name="actor_id"
            defaultValue={query.actor_id}
            placeholder="User UUID"
            className="mt-2 block h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground lg:w-44"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Source service
          <input
            type="text"
            name="source_service"
            defaultValue={query.source_service}
            placeholder="identity-service"
            className="mt-2 block h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground lg:w-44"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          From
          <input
            type="date"
            name="from"
            defaultValue={query.from}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          />
        </label>
        <label className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
          To
          <input
            type="date"
            name="to"
            defaultValue={query.to}
            className="mt-2 block h-10 rounded-md border border-border bg-background px-3 text-sm font-normal tracking-normal text-foreground"
          />
        </label>
        <div className="flex gap-2">
          <button
            type="submit"
            className="h-10 rounded-md bg-primary px-5 text-sm font-bold text-primary-foreground transition hover:-translate-y-0.5 hover:shadow-md"
          >
            Apply filters
          </button>
          {filtersActive || query.cursor ? (
            <Link
              href="/admin/audit"
              className="grid h-10 place-items-center rounded-md border border-border px-4 text-sm font-semibold text-muted-foreground transition hover:text-foreground"
            >
              Reset
            </Link>
          ) : null}
        </div>
      </form>

      {error ? (
        <EmptyState
          title="Could not load audit logs"
          description={error}
          icon={<ScrollText className="size-8" />}
        />
      ) : (
        <>
          <DataTable
            caption="Audit log events"
            rows={logs}
            keyExtractor={(log) => log.id}
            columns={[
              {
                key: "occurred_at",
                header: "Occurred at",
                cell: (log) => (
                  <span className="whitespace-nowrap text-sm">{formatTime(log.occurred_at)}</span>
                ),
              },
              {
                key: "event_type",
                header: "Event type",
                cell: (log) => <span className="font-mono text-xs">{log.event_type}</span>,
              },
              {
                key: "source_service",
                header: "Source service",
                cell: (log) => {
                  const service = rowField(log, "source_service");
                  return service ? (
                    <span className="font-mono text-xs">{service}</span>
                  ) : (
                    <span className="text-muted-foreground">Not recorded</span>
                  );
                },
              },
              {
                key: "actor_id",
                header: "Actor",
                cell: (log) =>
                  log.actor_id ? (
                    <span className="font-mono text-xs">{log.actor_id.slice(0, 8)}…</span>
                  ) : (
                    <span className="text-muted-foreground">System</span>
                  ),
              },
              {
                key: "action",
                header: "Action",
                cell: (log) => {
                  const action = rowField(log, "action");
                  return action ? (
                    <span className="capitalize">{action.replaceAll("_", " ")}</span>
                  ) : (
                    <span className="text-muted-foreground">
                      {log.resource_type
                        ? `${log.resource_type}${log.resource_id ? " updated" : ""}`
                        : "—"}
                    </span>
                  );
                },
              },
            ]}
            empty={
              <EmptyState
                title={filtersActive ? "No events match these filters" : "No audit events yet"}
                description={
                  filtersActive
                    ? "Widen the time range or clear a filter to see more of the trail."
                    : "Privileged actions across school services will appear here as they happen."
                }
                icon={<ScrollText className="size-8" />}
              />
            }
          />
          {nextCursor ? (
            <div className="flex justify-end">
              <Link
                href={`/admin/audit?${nextParams}`}
                className="rounded-md border border-border bg-surface px-4 py-2 text-sm font-semibold text-primary transition hover:-translate-y-0.5 hover:shadow-md"
              >
                Older events →
              </Link>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
