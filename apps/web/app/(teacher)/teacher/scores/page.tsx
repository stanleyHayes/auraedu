import Link from "next/link";
import { Trophy } from "lucide-react";
import { cn, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { TeacherScoresClient } from "@/components/teacher-scores-client";

export type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"] & {
  subject_name?: string;
  class_name?: string;
};

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type GradebookSummary = OpenAPI.assessment_v1.components["schemas"]["GradebookSummary"];

function formatPercent(value: number | null | undefined): string {
  return value == null ? "—" : value.toFixed(1);
}

export default async function TeacherScoresPage({
  searchParams,
}: {
  searchParams: Promise<{ class_id?: string }>;
}) {
  const { class_id: requestedClassId } = await searchParams;
  const client = await createServerClient();
  const [assessmentResult, classResult, subjectResult] = await Promise.allSettled([
    client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments?limit=100",
    ),
    client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>("/api/v1/classes?limit=100"),
    client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    ),
  ]);
  const classes: AcademicClass[] =
    classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];
  const subjects: Subject[] =
    subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
  const subjectName = new Map(subjects.map((subject) => [subject.id, subject.name]));
  const className = new Map(classes.map((academicClass) => [academicClass.id, academicClass.name]));
  const assessments: Assessment[] = (
    assessmentResult.status === "fulfilled" ? (assessmentResult.value.data ?? []) : []
  ).map((assessment) => ({
    ...assessment,
    subject_name: subjectName.get(assessment.subject_id),
    class_name: assessment.class_id ? className.get(assessment.class_id) : undefined,
  }));
  const selectedClassId = classes.some((academicClass) => academicClass.id === requestedClassId)
    ? requestedClassId
    : classes[0]?.id;

  let gradebook: GradebookSummary | null = null;
  let gradebookError: string | null = null;
  if (selectedClassId && classResult.status === "fulfilled") {
    try {
      gradebook = await client.get<GradebookSummary>(
        `/api/v1/gradebook?class_id=${encodeURIComponent(selectedClassId)}`,
      );
    } catch (error) {
      gradebookError = error instanceof Error ? error.message : "Failed to load gradebook";
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Trophy className="size-6" />}
        title="Scores"
        description="Review class performance and record scores for learners in your assigned classes."
      />

      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-sans font-semibold tracking-tight">Gradebook summary</h2>
          {classResult.status === "fulfilled" && classes.length > 0 ? (
            <nav aria-label="Gradebook class" className="flex flex-wrap gap-2">
              {classes.map((academicClass) => (
                <Link
                  key={academicClass.id}
                  href={`/teacher/scores?class_id=${academicClass.id}`}
                  className={cn(
                    "rounded-full border border-[var(--border)] px-3 py-1 text-sm",
                    academicClass.id === selectedClassId
                      ? "bg-[var(--primary)] text-[var(--primary-foreground)]"
                      : "bg-[var(--surface)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
                  )}
                >
                  {academicClass.name}
                </Link>
              ))}
            </nav>
          ) : null}
        </div>

        {gradebook ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              label="Overall average"
              value={formatPercent(gradebook.overall.average)}
              unit="%"
            />
            <StatCard
              label="Weighted average"
              value={formatPercent(gradebook.overall.weighted_average)}
              unit="%"
              tone="ok"
            />
            {gradebook.subjects.map((subject) => (
              <StatCard
                key={subject.subject_id}
                label={subjectName.get(subject.subject_id) ?? "Subject average"}
                value={formatPercent(subject.average)}
                unit="%"
              />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<Trophy className="size-8" />}
            title="No gradebook data"
            description={
              gradebookError ??
              (classResult.status === "rejected"
                ? "Assigned classes are temporarily unavailable."
                : classes.length === 0
                  ? "Gradebook summaries will appear once classes are assigned."
                  : "No gradebook summary is available for this class yet.")
            }
          />
        )}
      </section>

      <TeacherScoresClient
        assessments={assessments}
        assessmentsAvailable={assessmentResult.status === "fulfilled"}
      />
    </div>
  );
}
