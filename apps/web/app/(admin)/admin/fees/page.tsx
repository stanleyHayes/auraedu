import { BadgeDollarSign } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

function money(cents: number, currency: string) {
  return new Intl.NumberFormat("en-GH", { style: "currency", currency }).format(cents / 100);
}
export default async function AdminFeesPage() {
  const client = await createServerClient();
  const [structureResult, invoiceResult] = await Promise.allSettled([
    client.get<OpenAPI.fees_v1.components["schemas"]["FeeStructureList"]>(
      "/api/v1/fee-structures?limit=100",
    ),
    client.get<OpenAPI.fees_v1.components["schemas"]["InvoiceList"]>("/api/v1/invoices?limit=100"),
  ]);
  if (structureResult.status === "rejected" && invoiceResult.status === "rejected")
    return (
      <EmptyState
        icon={<BadgeDollarSign className="size-8" />}
        title="Fees unavailable"
        description="Fee structures and invoices could not be loaded."
      />
    );
  const structures = structureResult.status === "fulfilled" ? structureResult.value.data : [];
  const invoices = invoiceResult.status === "fulfilled" ? invoiceResult.value.data : [];
  const open = invoices.filter((invoice) =>
    ["pending", "partial", "overdue"].includes(invoice.status),
  );
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BadgeDollarSign className="size-6" />}
        title="Fees"
        description="Review school fee structures and invoice exposure."
      />
      <section className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Active structures"
          value={
            structureResult.status === "fulfilled"
              ? structures.filter((item) => item.status === "active").length
              : "—"
          }
          unit="configured"
        />
        <StatCard
          label="Open invoices"
          value={invoiceResult.status === "fulfilled" ? open.length : "—"}
          unit="attention"
          tone="warn"
        />
        <StatCard
          label="Overdue"
          value={
            invoiceResult.status === "fulfilled"
              ? invoices.filter((invoice) => invoice.status === "overdue").length
              : "—"
          }
          unit="invoices"
        />
      </section>
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <DataTable
          caption="Fee structures"
          rows={structures}
          keyExtractor={(item) => item.id}
          columns={[
            {
              key: "name",
              header: "Structure",
              cell: (item) => <span className="font-semibold">{item.name}</span>,
            },
            {
              key: "amount",
              header: "Amount",
              cell: (item) => money(item.amount_cents, item.currency),
            },
            {
              key: "recurrence",
              header: "Recurrence",
              cell: (item) => (
                <span className="capitalize">{item.recurrence.replaceAll("_", " ")}</span>
              ),
            },
            {
              key: "status",
              header: "Status",
              cell: (item) => <span className="capitalize">{item.status}</span>,
            },
          ]}
          empty={
            <EmptyState
              icon={<BadgeDollarSign className="size-8" />}
              title="No fee structures"
              description="Fee structures will appear once finance configures them."
            />
          }
        />
      </section>
    </div>
  );
}
