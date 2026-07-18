import { FileText, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { publicApiUrl } from "@auraedu/config";
import { createServerClient } from "@/lib/api";

// The live report-service DTO is ahead of the published contract: status and
// pdf_path/generated_at (set only once published) are not in report_v1 yet, and
// academic_year_id/template_id may be omitted on event-created drafts.
type ReportCard = OpenAPI.report_v1.components["schemas"]["ReportCard"] & {
  status?: string;
  academic_year_id?: string;
  template_id?: string;
  pdf_path?: string | null;
  created_at?: string;
  updated_at?: string;
};

export default async function ParentReportsPage() {
  let cards: ReportCard[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    // Parents read their own children's records; scoping is enforced server-side.
    const list = await client.get<{ data?: ReportCard[]; next_cursor?: string | null }>(
      "/api/v1/report-cards",
    );
    cards = list.data ?? [];
  } catch {
    error = "Could not load report cards right now.";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FileText className="size-6" />}
        title="Report Cards"
        description="Download and view your children's report cards."
      />
      {error ? (
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="Report cards unavailable"
          description={error}
        />
      ) : (
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h3 className="font-sans font-semibold tracking-tight">Term report cards</h3>
          <div className="mt-4">
            <DataTable
              caption="Children's report cards"
              columns={[
                { key: "student_id", header: "Student ID", cell: (c) => c.student_id },
                { key: "term_id", header: "Term", cell: (c) => c.term_id ?? "—" },
                {
                  key: "status",
                  header: "Status",
                  cell: (c) => <span className="capitalize">{c.status ?? "draft"}</span>,
                },
                {
                  key: "generated_at",
                  header: "Generated",
                  cell: (c) =>
                    c.generated_at ? new Date(c.generated_at).toLocaleDateString() : "—",
                },
                {
                  key: "download",
                  header: "",
                  cell: (c) =>
                    c.status === "published" ? (
                      <a
                        href={`${publicApiUrl}/api/v1/report-cards/${c.id}/download`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-sm font-medium text-[var(--primary)] hover:underline"
                      >
                        Download PDF
                      </a>
                    ) : null,
                },
              ]}
              rows={cards}
              keyExtractor={(c) => c.id}
              empty={
                <EmptyState
                  icon={<ClipboardList className="size-8" />}
                  title="No report cards"
                  description="Report cards will appear here once they are published by the school."
                />
              }
            />
          </div>
        </section>
      )}
    </div>
  );
}
