import { Receipt } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function SubscriptionsPage() {
  await requireAuth();

  let subscriptions: OpenAPI.billing_v1.components["schemas"]["Subscription"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{
      data?: OpenAPI.billing_v1.components["schemas"]["Subscription"][];
    }>("/api/v1/billing/subscriptions?limit=50");
    subscriptions = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load subscriptions";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Receipt className="size-7" />}
        title="Subscriptions"
        description="View tenant subscription status and billing periods."
      />

      {error ? (
        <EmptyState
          title="Could not load subscriptions"
          description={error}
          icon={<Receipt className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Subscriptions"
          rows={subscriptions}
          keyExtractor={(s) => s.id}
          columns={[
            {
              key: "tenant",
              header: "Tenant ID",
              cell: (s) => <span className="font-mono text-xs">{s.tenant_id}</span>,
            },
            {
              key: "plan",
              header: "Plan",
              cell: (s) => <span className="font-mono text-xs">{s.plan_key}</span>,
            },
            {
              key: "status",
              header: "Status",
              cell: (s) => <span className="capitalize">{s.status}</span>,
            },
            {
              key: "period",
              header: "Current period",
              cell: (s) => {
                const start = s.current_period_start
                  ? new Date(s.current_period_start).toLocaleDateString()
                  : "—";
                const end = s.current_period_end
                  ? new Date(s.current_period_end).toLocaleDateString()
                  : "—";
                return `${start} → ${end}`;
              },
            },
          ]}
          empty={
            <EmptyState
              title="No subscriptions yet"
              description="Subscriptions will appear here once tenants are enrolled."
              icon={<Receipt className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
