import { ScrollText } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function AuditLogsPage() {
  await requireAuth();

  let logs: OpenAPI.audit_v1.components["schemas"]["AuditLog"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OpenAPI.audit_v1.components["schemas"]["AuditLog"][] }>(
      "/api/v1/audit/logs?limit=50",
    );
    logs = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load audit logs";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ScrollText className="size-7" />}
        title="Audit logs"
        description="Review recent platform activity and changes."
      />

      {error ? (
        <EmptyState
          title="Could not load audit logs"
          description={error}
          icon={<ScrollText className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Audit logs"
          rows={logs}
          keyExtractor={(l) => l.id}
          columns={[
            {
              key: "time",
              header: "Time",
              cell: (l) => (
                <span className="font-mono text-xs">
                  {new Date(l.occurred_at).toLocaleString()}
                </span>
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
              key: "resource",
              header: "Resource",
              cell: (l) =>
                l.resource_type ? `${l.resource_type}${l.resource_id ? `:${l.resource_id}` : ""}` : "—",
            },
            {
              key: "tenant",
              header: "Tenant ID",
              cell: (l) => <span className="font-mono text-xs">{l.tenant_id}</span>,
            },
          ]}
          empty={
            <EmptyState
              title="No audit logs yet"
              description="Audit events will appear here once services start emitting them."
              icon={<ScrollText className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
