import { Receipt } from "lucide-react";
import { PageHeader, DataTable, EmptyState, Reveal, Watermark } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { SuperadminSubscriptionActions } from "@/components/superadmin-subscription-actions";

export default async function SubscriptionsPage() {
  await requireAuth();

  type Subscription = OpenAPI.billing_v1.components["schemas"]["Subscription"];
  type Plan = OpenAPI.billing_v1.components["schemas"]["Plan"];

  let subscriptions: Subscription[] = [];
  let plans: Plan[] = [];
  let plansByID = new Map<string, Plan>();
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [subscriptionPage, planPage] = await Promise.all([
      client.get<{ data?: Subscription[] }>("/api/v1/billing/subscriptions?limit=50"),
      client.get<{ data?: Plan[] }>("/api/v1/billing/plans?limit=100"),
    ]);
    subscriptions = subscriptionPage.data ?? [];
    plans = planPage.data ?? [];
    plansByID = new Map(plans.map((plan) => [plan.id, plan]));
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load subscriptions";
  }

  return (
    <div className="relative space-y-6">
      <Watermark className="pointer-events-none absolute -right-6 -top-10 text-[10rem] opacity-[0.03]">
        Billing
      </Watermark>
      <Reveal>
        <PageHeader
          icon={<Receipt className="size-7" />}
          title="Subscriptions"
          description="View tenant subscription status, change plans, and manage lifecycle state."
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Could not load subscriptions"
          description={error}
          icon={<Receipt className="size-8" />}
        />
      ) : (
        <Reveal delay={80}>
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
                cell: (s) => {
                  const plan = plansByID.get(s.plan_id);
                  return (
                    <span title={s.plan_id}>
                      {plan?.name ?? <span className="font-mono text-xs">{s.plan_id}</span>}
                    </span>
                  );
                },
              },
              {
                key: "status",
                header: "Status",
                className: "w-28",
                cell: (s) =>
                  s.status === "active" || s.status === "trialing" ? (
                    <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                      {s.status.replace("_", " ")}
                    </span>
                  ) : s.status === "past_due" ? (
                    <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                      past due
                    </span>
                  ) : (
                    <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                      {s.status}
                    </span>
                  ),
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
              {
                key: "actions",
                header: "Actions",
                className: "w-20",
                cell: (s) => (
                  <SuperadminSubscriptionActions
                    subscriptionId={s.id}
                    tenantLabel={s.tenant_id}
                    currentPlanId={s.plan_id}
                    currentStatus={s.status}
                    plans={plans.map((plan) => ({
                      id: plan.id,
                      name: plan.name,
                      code: plan.code,
                    }))}
                  />
                ),
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
        </Reveal>
      )}
    </div>
  );
}
