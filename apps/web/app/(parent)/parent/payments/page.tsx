import { Banknote, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Payment = OpenAPI.payment_v1.components["schemas"]["Payment"];

function formatMoney(amountCents: number, currency: string) {
  try {
    return new Intl.NumberFormat("en-GH", {
      style: "currency",
      currency,
    }).format(amountCents / 100);
  } catch {
    return `${currency} ${(amountCents / 100).toFixed(2)}`;
  }
}

export default async function ParentPaymentsPage() {
  const client = await createServerClient();
  let payments: Payment[];
  try {
    const res = await client.get<OpenAPI.payment_v1.components["schemas"]["PaymentList"]>(
      "/api/v1/payments?limit=50",
    );
    payments = res.data ?? [];
  } catch {
    payments = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Banknote className="size-6" />}
        title="Payments"
        description="View payments you have made to the school."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Payment history</h3>
        <div className="mt-4">
          <DataTable
            caption="Payments made to the school"
            columns={[
              {
                key: "amount",
                header: "Amount",
                cell: (r) => formatMoney(r.amount_cents, r.currency),
              },
              {
                key: "paid_at",
                header: "Date",
                cell: (r) => new Date(r.completed_at ?? r.initiated_at).toLocaleDateString("en-GH"),
              },
              {
                key: "method",
                header: "Provider",
                cell: (r) => <span className="capitalize">{r.provider}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (r) => <span className="capitalize">{r.status}</span>,
              },
            ]}
            rows={payments}
            keyExtractor={(r) => r.id}
            empty={
              <EmptyState
                icon={<ClipboardList className="size-8" />}
                title="No payments"
                description="Payments will appear here once you have made a fee payment."
              />
            }
          />
        </div>
      </section>
    </div>
  );
}
