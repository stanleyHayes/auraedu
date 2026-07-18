import { CreditCard, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

// amount is a display-only legacy field; the contract Invoice carries amount_due/amount_paid.
type Invoice = OpenAPI.fees_v1.components["schemas"]["Invoice"] & {
  amount?: number;
};

export default async function ParentFeesPage() {
  const client = await createServerClient();
  let invoices: Invoice[];
  try {
    const res = await client.get<OpenAPI.fees_v1.components["schemas"]["InvoiceList"]>(
      "/api/v1/invoices",
    );
    invoices = res.data ?? [];
  } catch {
    invoices = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CreditCard className="size-6" />}
        title="Fees"
        description="View invoices and outstanding fees for your children."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-sans font-semibold tracking-tight">Invoices</h3>
        <div className="mt-4">
          <DataTable
            caption="Invoices for your children"
            columns={[
              { key: "student_id", header: "Student ID", cell: (r) => r.student_id },
              { key: "amount", header: "Amount", cell: (r) => r.amount },
              { key: "due_date", header: "Due date", cell: (r) => r.due_date },
              {
                key: "status",
                header: "Status",
                cell: (r) => <span className="capitalize">{r.status}</span>,
              },
            ]}
            rows={invoices}
            keyExtractor={(r) => r.id}
            empty={
              <EmptyState
                icon={<ClipboardList className="size-8" />}
                title="No invoices"
                description="Invoices will appear here once fees are issued by the school."
              />
            }
          />
        </div>
      </section>
    </div>
  );
}
