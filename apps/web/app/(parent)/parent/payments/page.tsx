import { Banknote, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

export interface Payment {
  id: string;
  amount: number;
  paid_at: string;
  method: string;
  status: string;
}

export default async function ParentPaymentsPage() {
  const client = await createServerClient();
  let payments: Payment[];
  try {
    payments = await client.get<Payment[]>("/api/v1/payments");
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
              { key: "amount", header: "Amount", cell: (r) => r.amount },
              { key: "paid_at", header: "Date", cell: (r) => r.paid_at },
              { key: "method", header: "Method", cell: (r) => r.method },
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
