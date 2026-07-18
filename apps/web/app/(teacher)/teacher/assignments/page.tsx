import { ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState, type DataTableColumn } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

// The assessment-service treats `type` as a free-form string (the contract enum
// has no "assignment" value); subject_name/date are display-only legacy fields.
type Assignment = Omit<OpenAPI.assessment_v1.components["schemas"]["Assessment"], "type"> & {
  type: string;
  subject_name?: string;
  date?: string;
};

const columns: DataTableColumn<Assignment>[] = [
  { key: "name", header: "Assignment", cell: (a) => a.name },
  { key: "subject", header: "Subject", cell: (a) => a.subject_name ?? "—" },
  { key: "date", header: "Date", cell: (a) => a.date ?? "—" },
];

export default async function TeacherAssignmentsPage() {
  const client = await createServerClient();
  let assignments: Assignment[];
  try {
    const res = await client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments",
    );
    const data: Assignment[] = res.data ?? [];
    assignments = data.filter((a) => a.type === "assignment");
  } catch {
    assignments = [];
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ClipboardList className="size-6" />}
        title="Assignments"
        description="Assignments set for your classes."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <DataTable
          caption="Assignments filtered by type assignment"
          columns={columns}
          rows={assignments}
          keyExtractor={(a) => a.id}
          empty={
            <EmptyState
              icon={<ClipboardList className="size-8" />}
              title="No assignments"
              description="No assignments of type assignment were found."
            />
          }
        />
      </section>
    </div>
  );
}
