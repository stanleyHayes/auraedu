import { CreditCard } from "lucide-react";
import { PageHeader, DataTable, EmptyState, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { SuperadminPlanSheet } from "@/components/superadmin-plan-sheet";
import { SuperadminDeletePlanButton } from "@/components/superadmin-delete-plan-button";

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
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Plans
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<CreditCard className="size-7" />}
          title="Billing plans"
          description="Create and maintain the SaaS plans schools subscribe to."
          action={<SuperadminPlanSheet mode="create" />}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Could not load billing plans"
          description={error}
          icon={<CreditCard className="size-8" />}
        />
      ) : (
        <Reveal delay={80}>
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
                cell: (p) => (
                  <div>
                    <div className="font-medium">{p.name}</div>
                    {p.description ? (
                      <div className="mt-0.5 text-xs text-[var(--muted-foreground)]">
                        {p.description}
                      </div>
                    ) : null}
                  </div>
                ),
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
              {
                key: "status",
                header: "Status",
                className: "w-24",
                cell: (p) =>
                  p.status === "active" ? (
                    <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                      {p.status}
                    </span>
                  ) : (
                    <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                      {p.status ?? "—"}
                    </span>
                  ),
              },
              {
                key: "actions",
                header: "Actions",
                className: "w-24",
                cell: (p) => (
                  <div className="flex items-center gap-2">
                    <SuperadminPlanSheet
                      mode="edit"
                      plan={{
                        id: p.id,
                        code: p.code,
                        name: p.name,
                        description: p.description,
                        price_cents: p.price_cents,
                        currency: p.currency,
                        billing_interval: p.billing_interval,
                        features: p.features,
                        status: p.status,
                      }}
                    />
                    <SuperadminDeletePlanButton planId={p.id} name={p.name} />
                  </div>
                ),
              },
            ]}
            empty={
              <EmptyState
                title="No billing plans yet"
                description="Create the first plan to start subscribing schools."
                icon={<CreditCard className="size-8" />}
              />
            }
          />
        </Reveal>
      )}
    </div>
  );
}
