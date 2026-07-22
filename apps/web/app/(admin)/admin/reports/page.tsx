import { FileText } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function AdminReportsPage() {
  const client = await createServerClient();
  try {
    const list = await client.get<OpenAPI.report_v1.components["schemas"]["ReportCardList"]>(
      "/api/v1/report-cards?limit=100",
    );
    const rows = list.data;
    const count = (status: string) => rows.filter((card) => card.status === status).length;
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<FileText className="size-6" />}
          title="Report cards"
          description="Monitor report-card preparation, generation and publication."
        />
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Draft" value={count("draft")} unit="loaded cards" tone="warn" />
          <StatCard label="Generating" value={count("generating")} unit="PDF jobs" />
          <StatCard label="Published" value={count("published")} unit="ready" tone="ok" />
          <StatCard label="Archived" value={count("archived")} unit="retained" />
        </section>
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <DataTable
            caption="Report cards"
            rows={rows}
            keyExtractor={(card) => card.id}
            columns={[
              {
                key: "student",
                header: "Student ID",
                cell: (card) => <span className="font-mono text-xs">{card.student_id}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (card) => <span className="capitalize">{card.status}</span>,
              },
              {
                key: "updated",
                header: "Updated",
                cell: (card) => new Date(card.updated_at).toLocaleDateString("en-GB"),
              },
              {
                key: "download",
                header: "",
                cell: (card) =>
                  card.status === "published" ? (
                    <a
                      className="font-bold text-[var(--primary)] hover:underline"
                      href={`/api/reports/${card.id}/download`}
                    >
                      Download
                    </a>
                  ) : null,
              },
            ]}
            empty={
              <EmptyState
                icon={<FileText className="size-8" />}
                title="No report cards"
                description="Report cards will appear once preparation begins."
              />
            }
          />
        </section>
      </div>
    );
  } catch {
    return (
      <EmptyState
        icon={<FileText className="size-8" />}
        title="Report cards unavailable"
        description="The reporting service could not be reached."
      />
    );
  }
}
