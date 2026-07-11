import { Building2 } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function TenantsPage() {
  await requireAuth();

  let tenants: OpenAPI.tenant_v1.components["schemas"]["Tenant"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OpenAPI.tenant_v1.components["schemas"]["Tenant"][] }>(
      "/api/v1/tenants?limit=50",
    );
    tenants = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load tenants";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Building2 className="size-7" />}
        title="Tenants"
        description="View all schools and organisations on the platform."
      />

      {error ? (
        <EmptyState
          title="Could not load tenants"
          description={error}
          icon={<Building2 className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Tenants"
          rows={tenants}
          keyExtractor={(t) => t.tenant_code}
          columns={[
            {
              key: "code",
              header: "Code",
              cell: (t) => <span className="font-mono text-xs">{t.tenant_code}</span>,
            },
            {
              key: "name",
              header: "Name",
              cell: (t) => t.name,
            },
            {
              key: "status",
              header: "Status",
              cell: (t) => <span className="capitalize">{t.status}</span>,
            },
            {
              key: "plan",
              header: "Plan",
              cell: (t) => <span className="capitalize">{t.plan ?? "—"}</span>,
            },
            {
              key: "domain",
              header: "Domain",
              cell: (t) => t.domain ?? "—",
            },
          ]}
          empty={
            <EmptyState
              title="No tenants yet"
              description="Tenants will appear here once schools are onboarded."
              icon={<Building2 className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
