import { ClipboardCheck } from "lucide-react";
import { DataTable, EmptyState, PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

export default async function AdminAssessmentsPage() {
  const client = await createServerClient();
  const [assessmentResult, subjectResult, classResult] = await Promise.allSettled([
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>("/api/v1/classes?limit=100"),
  ]);
  if (assessmentResult.status === "rejected")
    return (
      <EmptyState
        icon={<ClipboardCheck className="size-8" />}
        title="Assessments unavailable"
        description="The assessment catalogue could not be loaded."
      />
    );
  const subjects = new Map(
    (subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : []).map(
      (subject) => [subject.id, subject.name],
    ),
  );
  const classes = new Map(
    (classResult.status === "fulfilled" ? (classResult.value.data ?? []) : []).map(
      (academicClass) => [academicClass.id, academicClass.name],
    ),
  );
  const rows = assessmentResult.value.data ?? [];
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ClipboardCheck className="size-6" />}
        title="Assessments"
        description="Review the assessment catalogue across the school."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <DataTable
          caption="School assessments"
          rows={rows}
          keyExtractor={(assessment) => assessment.id}
          columns={[
            {
              key: "name",
              header: "Assessment",
              cell: (assessment) => <span className="font-semibold">{assessment.name}</span>,
            },
            {
              key: "type",
              header: "Type",
              cell: (assessment) => <span className="capitalize">{assessment.type}</span>,
            },
            {
              key: "subject",
              header: "Subject",
              cell: (assessment) => subjects.get(assessment.subject_id) ?? "Subject unavailable",
            },
            {
              key: "class",
              header: "Class",
              cell: (assessment) =>
                assessment.class_id
                  ? (classes.get(assessment.class_id) ?? "Class unavailable")
                  : "Not assigned",
            },
            {
              key: "maximum",
              header: "Maximum",
              cell: (assessment) => assessment.max_score ?? "Not set",
            },
          ]}
          empty={
            <EmptyState
              icon={<ClipboardCheck className="size-8" />}
              title="No assessments"
              description="Assessments will appear once academic staff create them."
            />
          }
        />
      </section>
    </div>
  );
}
