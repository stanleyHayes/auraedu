import { CreditCard, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Invoice = OpenAPI.fees_v1.components["schemas"]["Invoice"];
type FeeStructure = OpenAPI.fees_v1.components["schemas"]["FeeStructure"];

function formatMoney(amountCents: number, currency: string) {
  try {
    return new Intl.NumberFormat("en-GH", { style: "currency", currency }).format(
      amountCents / 100,
    );
  } catch {
    return `${currency} ${(amountCents / 100).toFixed(2)}`;
  }
}

export default async function ParentFeesPage() {
  const client = await createServerClient();
  let invoices: Invoice[] = [];
  let structures: Record<string, FeeStructure> = {};
  let students: Record<string, string> = {};
  let error: string | null = null;
  try {
    const [invoiceList, structureList, family] = await Promise.all([
      client.get<OpenAPI.fees_v1.components["schemas"]["InvoiceList"]>("/api/v1/invoices"),
      client.get<OpenAPI.fees_v1.components["schemas"]["FeeStructureList"]>(
        "/api/v1/fee-structures?limit=100",
      ),
      client.get<OpenAPI.student_v1.components["schemas"]["GuardianChildren"]>(
        "/api/v1/guardians/me/children",
      ),
    ]);
    invoices = invoiceList.data ?? [];
    structures = Object.fromEntries((structureList.data ?? []).map((item) => [item.id, item]));
    students = Object.fromEntries(
      family.students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
    );
  } catch {
    error = "Invoices for your linked children could not be loaded right now.";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CreditCard className="size-6" />}
        title="Fees"
        description="View invoices and outstanding fees for your children."
      />
      {error ? (
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="Invoices unavailable"
          description={error}
        />
      ) : (
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-sans font-semibold tracking-tight">Invoices</h3>
          <div className="mt-4">
            <DataTable
              caption="Invoices for your children"
              columns={[
                {
                  key: "student_id",
                  header: "Learner",
                  cell: (r) => students[r.student_id] ?? "Linked learner",
                },
                {
                  key: "amount",
                  header: "Amount",
                  cell: (r) =>
                    formatMoney(r.amount_cents, structures[r.fee_structure_id]?.currency ?? "GHS"),
                },
                {
                  key: "balance",
                  header: "Outstanding",
                  cell: (r) =>
                    formatMoney(r.balance_cents, structures[r.fee_structure_id]?.currency ?? "GHS"),
                },
                {
                  key: "due_date",
                  header: "Due date",
                  cell: (r) => (r.due_date?.trim() ? r.due_date : "—"),
                },
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
      )}
    </div>
  );
}
