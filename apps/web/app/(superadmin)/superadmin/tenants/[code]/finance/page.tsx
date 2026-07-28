import { Wallet } from "lucide-react";
import { PageHeader, DataTable, EmptyState, StatCard, Reveal } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClientForTenant } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { drilldownQuery, nextPageHref, formatDateTime } from "@/lib/superadmin-drilldown";
import { SuperadminPagination } from "@/components/superadmin-pagination";

type Invoice = OpenAPI.fees_v1.components["schemas"]["Invoice"];
type Payment = OpenAPI.payment_v1.components["schemas"]["Payment"];

interface TenantFinancePageProps {
  params: Promise<{ code: string }>;
  searchParams: Promise<{ cursor?: string }>;
}

function formatMoney(cents: number, currency: string): string {
  try {
    return new Intl.NumberFormat("en-GH", { style: "currency", currency }).format(cents / 100);
  } catch {
    return `${(cents / 100).toFixed(2)} ${currency}`;
  }
}

export default async function TenantFinancePage({ params, searchParams }: TenantFinancePageProps) {
  const { code } = await params;
  const { cursor } = await searchParams;
  await requireAuth();

  let invoices: Invoice[] = [];
  let payments: Payment[] = [];
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClientForTenant(code);
    const [invoicePage, paymentPage] = await Promise.all([
      client.get<{ data?: Invoice[]; next_cursor?: string | null }>(
        `/api/v1/invoices?${drilldownQuery({}, cursor)}`,
      ),
      client.get<{ data?: Payment[] }>("/api/v1/payments?limit=10"),
    ]);
    invoices = invoicePage.data ?? [];
    payments = paymentPage.data ?? [];
    nextCursor = invoicePage.next_cursor ?? null;
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load finance data";
  }

  const countByStatus = (status: Invoice["status"]) =>
    invoices.filter((i) => i.status === status).length;
  const openCount = countByStatus("pending") + countByStatus("partial");
  const nextHref = nextPageHref(`/superadmin/tenants/${code}/finance`, {}, nextCursor);

  return (
    <div className="space-y-6">
      <Reveal>
        <PageHeader
          icon={<Wallet className="size-7" />}
          title="Finance"
          description={`Read-only fee invoices and payments for ${code}.`}
        />
      </Reveal>

      {error ? (
        <EmptyState
          title="Finance data unavailable"
          description={error}
          icon={<Wallet className="size-8" />}
        />
      ) : (
        <>
          <Reveal delay={60}>
            <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                label="Invoices"
                value={`${invoices.length}${nextCursor ? "+" : ""}`}
                unit="this page"
              />
              <StatCard label="Paid" value={countByStatus("paid")} unit="invoices" tone="ok" />
              <StatCard label="Open" value={openCount} unit="pending / partial" />
              <StatCard
                label="Overdue"
                value={countByStatus("overdue")}
                unit="invoices"
                tone={countByStatus("overdue") > 0 ? "warn" : "default"}
              />
            </section>
          </Reveal>

          <Reveal delay={100}>
            <section className="space-y-3">
              <h2 className="font-heading text-lg font-bold">Invoices</h2>
              <DataTable
                caption={`Invoices for tenant ${code}`}
                rows={invoices}
                keyExtractor={(i) => i.id}
                columns={[
                  {
                    key: "invoice",
                    header: "Invoice",
                    cell: (i) => (
                      <span className="font-mono text-xs" title={i.id}>
                        {i.id.slice(0, 8)}…
                      </span>
                    ),
                  },
                  {
                    key: "student",
                    header: "Student",
                    cell: (i) => (
                      <span className="font-mono text-xs" title={i.student_id}>
                        {i.student_id.slice(0, 8)}…
                      </span>
                    ),
                  },
                  {
                    key: "amount",
                    header: "Amount",
                    cell: (i) => formatMoney(i.amount_cents, "GHS"),
                  },
                  {
                    key: "balance",
                    header: "Balance",
                    cell: (i) => formatMoney(i.balance_cents, "GHS"),
                  },
                  {
                    key: "status",
                    header: "Status",
                    className: "w-28",
                    cell: (i) =>
                      i.status === "paid" ? (
                        <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                          {i.status}
                        </span>
                      ) : i.status === "overdue" ? (
                        <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                          {i.status}
                        </span>
                      ) : (
                        <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                          {i.status}
                        </span>
                      ),
                  },
                  {
                    key: "issued",
                    header: "Issued",
                    cell: (i) => (
                      <span className="font-mono text-xs">{formatDateTime(i.issued_at)}</span>
                    ),
                  },
                ]}
                empty={
                  <EmptyState
                    title="No invoices yet"
                    description="This school has not issued any fee invoices."
                    icon={<Wallet className="size-8" />}
                  />
                }
              />
              <SuperadminPagination nextHref={nextHref} />
            </section>
          </Reveal>

          <Reveal delay={140}>
            <section className="space-y-3">
              <h2 className="font-heading text-lg font-bold">Recent payments</h2>
              <DataTable
                caption={`Recent payments for tenant ${code}`}
                rows={payments}
                keyExtractor={(p) => p.id}
                columns={[
                  {
                    key: "payment",
                    header: "Payment",
                    cell: (p) => (
                      <span className="font-mono text-xs" title={p.id}>
                        {p.id.slice(0, 8)}…
                      </span>
                    ),
                  },
                  {
                    key: "amount",
                    header: "Amount",
                    cell: (p) => formatMoney(p.amount_cents, p.currency),
                  },
                  {
                    key: "provider",
                    header: "Provider",
                    cell: (p) => <span className="capitalize">{p.provider}</span>,
                  },
                  {
                    key: "status",
                    header: "Status",
                    className: "w-28",
                    cell: (p) =>
                      p.status === "success" ? (
                        <span className="rounded-full bg-[var(--color-ok)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-ok)]">
                          {p.status}
                        </span>
                      ) : p.status === "failed" || p.status === "cancelled" ? (
                        <span className="rounded-full bg-[var(--color-warn)]/10 px-2 py-0.5 text-xs capitalize text-[var(--color-warn)]">
                          {p.status}
                        </span>
                      ) : (
                        <span className="rounded-full bg-[var(--muted)] px-2 py-0.5 text-xs capitalize text-[var(--muted-foreground)]">
                          {p.status}
                        </span>
                      ),
                  },
                  {
                    key: "initiated",
                    header: "Initiated",
                    cell: (p) => (
                      <span className="font-mono text-xs">{formatDateTime(p.initiated_at)}</span>
                    ),
                  },
                ]}
                empty={
                  <EmptyState
                    title="No payments yet"
                    description="Payments will appear here once fees are collected."
                    icon={<Wallet className="size-8" />}
                  />
                }
              />
            </section>
          </Reveal>
        </>
      )}
    </div>
  );
}
