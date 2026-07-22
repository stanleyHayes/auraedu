import { FileText, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type ReportCard = OpenAPI.report_v1.components["schemas"]["ReportCard"];

export default async function ParentReportsPage() {
  let cards: ReportCard[] = [];
  let students: Record<string, string> = {};
  let terms: Record<string, string> = {};
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [list, family] = await Promise.all([
      client.get<{ data?: ReportCard[]; next_cursor?: string | null }>("/api/v1/report-cards"),
      client.get<OpenAPI.student_v1.components["schemas"]["GuardianChildren"]>(
        "/api/v1/guardians/me/children",
      ),
    ]);
    cards = list.data ?? [];
    students = Object.fromEntries(
      family.students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
    );
    try {
      const termList =
        await client.get<OpenAPI.academic_v1.components["schemas"]["TermList"]>(
          "/api/v1/terms?limit=100",
        );
      terms = Object.fromEntries((termList.data ?? []).map((term) => [term.id, term.name]));
    } catch {
      terms = {};
    }
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
                {
                  key: "student_id",
                  header: "Learner",
                  cell: (c) => (
                    <span className="font-semibold">
                      {students[c.student_id] ?? "Linked learner"}
                    </span>
                  ),
                },
                {
                  key: "term_id",
                  header: "Term",
                  cell: (c) => (c.term_id ? (terms[c.term_id] ?? "Published term") : "—"),
                },
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
                        href={`/api/reports/${c.id}/download`}
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
