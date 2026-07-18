import { ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { AssignmentFormSheet } from "@/components/assignment-form-sheet";
import { PublishAssignmentButton } from "@/components/publish-assignment-button";
import { DeleteAssignmentButton } from "@/components/delete-assignment-button";

type Assignment = OpenAPI.assessment_v1.components["schemas"]["Assignment"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

export default async function TeacherAssignmentsPage() {
  let assignments: Assignment[] = [];
  let subjects: Subject[] = [];
  let classes: AcademicClass[] = [];
  let years: AcademicYear[] = [];
  let error: string | null = null;

  const client = await createServerClient();

  try {
    const res = await client.get<OpenAPI.assessment_v1.components["schemas"]["AssignmentList"]>(
      "/api/v1/assignments?limit=50",
    );
    assignments = res.data ?? [];
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load assignments";
  }

  // Supplemental lookups are best-effort so the list still renders when a
  // related service or feature is unavailable.
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    );
    subjects = res.data ?? [];
  } catch {
    subjects = [];
  }

  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
      "/api/v1/classes?limit=100",
    );
    classes = res.data ?? [];
  } catch {
    classes = [];
  }

  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["AcademicYearList"]>(
      "/api/v1/academic-years?limit=50",
    );
    years = res.data ?? [];
  } catch {
    years = [];
  }

  const subjectName = new Map(subjects.map((s) => [s.id, s.name]));
  const className = new Map(classes.map((c) => [c.id, c.name]));

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ClipboardList className="size-6" />}
        title="Assignments"
        description="Assignments set for your classes."
        action={<AssignmentFormSheet mode="create" subjects={subjects} classes={classes} years={years} />}
      />

      {error ? (
        <EmptyState
          title="Could not load assignments"
          description={error}
          icon={<ClipboardList className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Assignments"
          rows={assignments}
          keyExtractor={(a) => a.id}
          columns={[
            {
              key: "title",
              header: "Title",
              cell: (a) => <span className="font-medium">{a.title}</span>,
            },
            {
              key: "subject",
              header: "Subject",
              cell: (a) => subjectName.get(a.subject_id) ?? "—",
            },
            {
              key: "classes",
              header: "Classes",
              cell: (a) =>
                a.class_ids && a.class_ids.length > 0
                  ? a.class_ids.map((id) => className.get(id) ?? "—").join(", ")
                  : "All",
            },
            {
              key: "due",
              header: "Due",
              cell: (a) => (a.due_date ? a.due_date.slice(0, 10) : "—"),
            },
            {
              key: "max_score",
              header: "Max score",
              cell: (a) => a.max_score ?? "—",
            },
            {
              key: "status",
              header: "Status",
              cell: (a) => <span className="capitalize">{a.status ?? "draft"}</span>,
            },
            {
              key: "actions",
              header: "Actions",
              className: "w-28",
              cell: (a) => (
                <div className="flex items-center gap-2">
                  <AssignmentFormSheet
                    mode="edit"
                    initial={a}
                    subjects={subjects}
                    classes={classes}
                    years={years}
                  />
                  {a.status !== "published" ? (
                    <PublishAssignmentButton id={a.id} title={a.title} />
                  ) : null}
                  <DeleteAssignmentButton id={a.id} title={a.title} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              icon={<ClipboardList className="size-8" />}
              title="No assignments"
              description="Assignments you create will appear here."
            />
          }
        />
      )}
    </div>
  );
}
