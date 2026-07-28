import { ScrollText } from "lucide-react";
import { PageHeader, DataTable, EmptyState, Reveal, Button, Input, Label } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import {
  drilldownQuery,
  nextPageHref,
  formatDateTime,
  dateToIsoStart,
  dateToIsoEnd,
  metadataString,
} from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type AuditLog = OpenAPI.audit_v1.components["schemas"]["AuditLog"];

interface TenantAuditPageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{
    event_type?: string;
    actor_id?: string;
    source_service?: string;
    from?: string;
    to?: string;
    cursor?: string;
  }>;
}

export default async function TenantAuditPage({ params, searchParams }: TenantAuditPageProps) {
  const { code } = await params;
  const filters = await searchParams;
  await requireAuth();

  let logs: AuditLog[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const res = await client.get<{ data?: AuditLog[]; next_cursor?: string | null }>(
      `/api/v1/audit/logs?${drilldownQuery(
        {
          event_type: filters.event_type,
          actor_id: filters.actor_id,
          source_service: filters.source_service,
          from: dateToIsoStart(filters.from),
          to: dateToIsoEnd(filters.to),
        },
        filters.cursor,
      )}`,
    );
    logs = res.data ?? [];
    nextCursor = res.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load audit logs";
  }

  const nextHref = nextPageHref(`/superadmin/tenants/${code}/audit`, filters, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<ScrollText className="size-7" />}
          title="Audit feed"
          description={`Read-only audit activity for ${code}.`}
        />
      </Reveal>

      <Reveal delay={40}>
        <form
          method="get"
          className="card grid gap-3 rounded-[var(--radius-md)] p-4 sm:grid-cols-2 lg:grid-cols-6"
        >
          <div className="space-y-1.5">
            <Label htmlFor="event_type">Event type</Label>
            <Input
              id="event_type"
              name="event_type"
              defaultValue={filters.event_type ?? ""}
              placeholder="e.g. student.created"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="actor_id">Actor ID</Label>
            <Input
              id="actor_id"
              name="actor_id"
              defaultValue={filters.actor_id ?? ""}
              placeholder="User UUID"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="source_service">Source service</Label>
            <Input
              id="source_service"
              name="source_service"
              defaultValue={filters.source_service ?? ""}
              placeholder="e.g. student-service"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="from">From</Label>
            <Input id="from" name="from" type="date" defaultValue={filters.from ?? ""} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="to">To</Label>
            <Input id="to" name="to" type="date" defaultValue={filters.to ?? ""} />
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
          title="Audit feed unavailable"
          description={error}
          icon={<ScrollText className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={80}>
            <DataTable
              caption={`Audit logs for tenant ${code}`}
              rows={logs}
              keyExtractor={(l) => l.id}
              columns={[
                {
                  key: "time",
                  header: "Time",
                  cell: (l) => (
                    <span className="font-mono text-xs">{formatDateTime(l.occurred_at)}</span>
                  ),
                },
                {
                  key: "event",
                  header: "Event",
                  cell: (l) => <span className="font-mono text-xs">{l.event_type}</span>,
                },
                {
                  key: "actor",
                  header: "Actor",
                  cell: (l) => <span className="font-mono text-xs">{l.actor_id ?? "system"}</span>,
                },
                {
                  key: "source",
                  header: "Source",
                  cell: (l) => (
                    <span className="font-mono text-xs">
                      {metadataString(l.metadata, "source_service")}
                    </span>
                  ),
                },
                {
                  key: "resource",
                  header: "Resource",
                  cell: (l) =>
                    l.resource_type
                      ? `${l.resource_type}${l.resource_id ? `:${l.resource_id}` : ""}`
                      : "—",
                },
              ]}
              empty={
                <EmptyState
                  title="No audit events"
                  description="No events match these filters for this school."
                  icon={<ScrollText className="size-8" />}
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
