import { CreditCard } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

function money(cents: number, currency: string) {
  return new Intl.NumberFormat("en-GH", { style: "currency", currency }).format(cents / 100);
}
export default async function AdminPaymentsPage() {
  const client = await createServerClient();
  try {
    const list = await client.get<OpenAPI.payment_v1.components["schemas"]["PaymentList"]>(
      "/api/v1/payments?limit=100",
    );
    const rows = list.data ?? [];
    const count = (status: string) => rows.filter((payment) => payment.status === status).length;
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<CreditCard className="size-6" />}
          title="Payments"
          description="Monitor payment attempts and provider outcomes."
        />
        <section className="grid gap-4 sm:grid-cols-3">
          <StatCard label="Successful" value={count("success")} unit="loaded payments" tone="ok" />
          <StatCard
            label="In progress"
            value={count("pending") + count("processing")}
            unit="payments"
            tone="warn"
          />
          <StatCard label="Failed" value={count("failed")} unit="payments" />
        </section>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <DataTable
            caption="Payments"
            rows={rows}
            keyExtractor={(payment) => payment.id}
            columns={[
              {
                key: "reference",
                header: "Reference",
                cell: (payment) => (
                  <span className="font-mono text-xs">
                    {payment.provider_reference ?? payment.id.slice(0, 8)}
                  </span>
                ),
              },
              {
                key: "amount",
                header: "Amount",
                cell: (payment) => money(payment.amount_cents, payment.currency),
              },
              {
                key: "provider",
                header: "Provider",
                cell: (payment) => <span className="capitalize">{payment.provider}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (payment) => <span className="capitalize">{payment.status}</span>,
              },
              {
                key: "initiated",
                header: "Initiated",
                cell: (payment) => new Date(payment.initiated_at).toLocaleString("en-GB"),
              },
            ]}
            empty={
              <EmptyState
                icon={<CreditCard className="size-8" />}
                title="No payments"
                description="Provider transactions will appear once payment attempts begin."
              />
            }
          />
        </section>
      </div>
    );
  } catch {
    return (
      <EmptyState
        icon={<CreditCard className="size-8" />}
        title="Payments unavailable"
        description="The payment service could not be reached."
      />
    );
  }
}
