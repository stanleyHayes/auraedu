import { BadgeDollarSign } from "lucide-react";
import { DataTable, EmptyState, PageHeader, Reveal, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminFeesStructureSheet } from "@/components/admin-fees-structure-sheet";
import { AdminFeesDeleteStructureButton } from "@/components/admin-fees-delete-structure-button";
import { AdminFeesInvoiceSheet } from "@/components/admin-fees-invoice-sheet";

function money(cents: number, currency: string) {
  return new Intl.NumberFormat("en-GH", { style: "currency", currency }).format(cents / 100);
}

const INVOICE_STATUS_STYLE: Record<string, string> = {
  paid: "bg-emerald-50 text-emerald-800",
  pending: "bg-amber-50 text-amber-800",
  partial: "bg-blue-50 text-blue-800",
  overdue: "bg-red-50 text-red-800",
  draft: "bg-muted text-muted-foreground",
  cancelled: "bg-muted text-muted-foreground",
};

export default async function AdminFeesPage() {
  await requireAuth();
  const client = await createServerClient();

  const [structureResult, invoiceResult, yearResult, studentResult, classResult] =
    await Promise.allSettled([
      client.get<OpenAPI.fees_v1.components["schemas"]["FeeStructureList"]>(
        "/api/v1/fee-structures?limit=100",
      ),
      client.get<OpenAPI.fees_v1.components["schemas"]["InvoiceList"]>(
        "/api/v1/invoices?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
        "/api/v1/academic-years?limit=50",
      ),
      client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        "/api/v1/students?limit=100",
      ),
      client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
        "/api/v1/classes?limit=50",
      ),
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
  const years = yearResult.status === "fulfilled" ? (yearResult.value.data ?? []) : [];
  const students = studentResult.status === "fulfilled" ? (studentResult.value.data ?? []) : [];
  const classes = classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];

  const yearName = new Map(years.map((year) => [year.id, year.name]));
  const structureCurrency = new Map(structures.map((item) => [item.id, item.currency]));
  const studentName = new Map(
    students.map((student) => [
      student.id,
      `${student.first_name} ${student.last_name}`.trim() || student.student_code,
    ]),
  );
  const open = invoices.filter((invoice) =>
    ["pending", "partial", "overdue"].includes(invoice.status),
  );

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BadgeDollarSign className="size-6" />}
        title="Fees"
        description="Configure fee structures for each academic year and issue invoices to students."
        action={
          <div className="flex flex-wrap gap-2">
            <AdminFeesInvoiceSheet
              structures={structures}
              classes={classes.map((item) => ({ id: item.id, name: item.name }))}
              students={students
                .filter((student) => student.status === "active")
                .map((student) => ({
                  id: student.id,
                  name: `${student.first_name} ${student.last_name}`.trim() || student.student_code,
                  classId: student.class_id ?? null,
                }))}
            />
            <AdminFeesStructureSheet mode="create" years={years} />
          </div>
        }
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

      <Reveal delay={70}>
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
                key: "year",
                header: "Academic year",
                cell: (item) => yearName.get(item.academic_year_id) ?? "—",
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
                key: "scope",
                header: "Scope",
                cell: (item) => (
                  <span>
                    {item.target === "all_students" ? "All students" : "Specific student"}
                    {item.due_day ? ` · day ${item.due_day}` : ""}
                  </span>
                ),
              },
              {
                key: "status",
                header: "Status",
                cell: (item) => <span className="capitalize">{item.status}</span>,
              },
              {
                key: "actions",
                header: "Actions",
                className: "w-24",
                cell: (item) => (
                  <div className="flex items-center gap-2">
                    <AdminFeesStructureSheet mode="edit" initial={item} years={years} />
                    <AdminFeesDeleteStructureButton id={item.id} name={item.name} />
                  </div>
                ),
              },
            ]}
            empty={
              <EmptyState
                icon={<BadgeDollarSign className="size-8" />}
                title="No fee structures"
                description="Create a fee structure to start invoicing students."
              />
            }
          />
        </section>
      </Reveal>

      <Reveal delay={120}>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <DataTable
            caption="Invoices"
            rows={invoices}
            keyExtractor={(invoice) => invoice.id}
            columns={[
              {
                key: "student",
                header: "Student",
                cell: (invoice) => (
                  <span className="font-semibold">
                    {studentName.get(invoice.student_id) ?? invoice.student_id}
                  </span>
                ),
              },
              {
                key: "amount",
                header: "Amount",
                cell: (invoice) =>
                  money(
                    invoice.amount_cents,
                    structureCurrency.get(invoice.fee_structure_id) ?? "GHS",
                  ),
              },
              {
                key: "balance",
                header: "Balance",
                cell: (invoice) =>
                  money(
                    invoice.balance_cents,
                    structureCurrency.get(invoice.fee_structure_id) ?? "GHS",
                  ),
              },
              {
                key: "due",
                header: "Due date",
                cell: (invoice) =>
                  invoice.due_date ? new Date(invoice.due_date).toLocaleDateString("en-GB") : "—",
              },
              {
                key: "status",
                header: "Status",
                cell: (invoice) => (
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${
                      INVOICE_STATUS_STYLE[invoice.status] ?? "bg-muted text-muted-foreground"
                    }`}
                  >
                    {invoice.status}
                  </span>
                ),
              },
              {
                key: "issued",
                header: "Issued",
                cell: (invoice) => new Date(invoice.issued_at).toLocaleDateString("en-GB"),
              },
            ]}
            empty={
              <EmptyState
                icon={<BadgeDollarSign className="size-8" />}
                title="No invoices yet"
                description="Issue an invoice from an active fee structure to bill students."
              />
            }
          />
        </section>
      </Reveal>
    </div>
  );
}
