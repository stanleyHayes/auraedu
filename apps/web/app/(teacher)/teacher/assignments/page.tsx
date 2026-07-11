import { ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState, type DataTableColumn } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

interface Assignment {
  id: string;
  name: string;
  type: string;
  subject_name?: string;
  date?: string;
}

const columns: DataTableColumn<Assignment>[] = [
  { key: "name", header: "Assignment", cell: (a) => a.name },
  { key: "subject", header: "Subject", cell: (a) => a.subject_name ?? "—" },
  { key: "date", header: "Date", cell: (a) => a.date ?? "—" },
];

export default async function TeacherAssignmentsPage() {
  const client = await createServerClient();
  let assignments: Assignment[];
  try {
    const assessments = await client.get<Assignment[]>("/api/v1/assessments");
    assignments = assessments.filter((a) => a.type === "assignment");
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
