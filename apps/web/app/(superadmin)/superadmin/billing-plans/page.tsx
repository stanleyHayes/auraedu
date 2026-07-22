import { CreditCard } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";

export default async function BillingPlansPage() {
  await requireAuth();

  let plans: OpenAPI.billing_v1.components["schemas"]["Plan"][] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const res = await client.get<{ data?: OpenAPI.billing_v1.components["schemas"]["Plan"][] }>(
      "/api/v1/billing/plans?limit=50",
    );
    plans = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load billing plans";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CreditCard className="size-7" />}
        title="Billing plans"
        description="View SaaS plans and their features."
      />

      {error ? (
        <EmptyState
          title="Could not load billing plans"
          description={error}
          icon={<CreditCard className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Billing plans"
          rows={plans}
          keyExtractor={(p) => p.id}
          columns={[
            {
              key: "code",
              header: "Code",
              cell: (p) => <span className="font-mono text-xs">{p.code}</span>,
            },
            {
              key: "name",
              header: "Name",
              cell: (p) => p.name,
            },
            {
              key: "price",
              header: "Price",
              cell: (p) =>
                `${new Intl.NumberFormat("en-GH", {
                  style: "currency",
                  currency: p.currency,
                }).format(
                  p.price_cents / 100,
                )} / ${p.billing_interval === "yearly" ? "year" : "month"}`,
            },
            {
              key: "features",
              header: "Features",
              cell: (p) => (p.features && p.features.length > 0 ? p.features.join(", ") : "—"),
            },
          ]}
          empty={
            <EmptyState
              title="No billing plans yet"
              description="Plans will appear here once billing is configured."
              icon={<CreditCard className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
