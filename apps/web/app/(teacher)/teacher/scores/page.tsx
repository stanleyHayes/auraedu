import Link from "next/link";
import { Trophy } from "lucide-react";
import { EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { TeacherScoresClient } from "@/components/teacher-scores-client";
import { cn } from "@auraedu/ui";

// subject_name/date are display-only legacy fields not present in the contract.
export type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"] & {
  subject_name?: string;
  date?: string;
};

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type GradebookSummary = OpenAPI.assessment_v1.components["schemas"]["GradebookSummary"];

function formatPercent(value: number | null | undefined): string {
  return value == null ? "—" : value.toFixed(1);
}

interface TeacherScoresPageProps {
  searchParams: Promise<{ class_id?: string }>;
}

export default async function TeacherScoresPage({ searchParams }: TeacherScoresPageProps) {
  const { class_id: requestedClassId } = await searchParams;
  const client = await createServerClient();

  let assessments: Assessment[];
  try {
    const res = await client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
      "/api/v1/assessments",
    );
    assessments = res.data ?? [];
  } catch {
    assessments = [];
  }

  // Supplemental lookups for the gradebook summary are best-effort so the
  // scores list still renders when a related service is unavailable.
  let classes: AcademicClass[];
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>(
      "/api/v1/classes?limit=100",
    );
    classes = res.data ?? [];
  } catch {
    classes = [];
  }

  let subjects: Subject[];
  try {
    const res = await client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    );
    subjects = res.data ?? [];
  } catch {
    subjects = [];
  }

  const subjectName = new Map(subjects.map((s) => [s.id, s.name]));
  const selectedClassId = requestedClassId ?? classes[0]?.id;

  let gradebook: GradebookSummary | null = null;
  let gradebookError: string | null = null;
  if (selectedClassId) {
    try {
      gradebook = await client.get<GradebookSummary>(
        `/api/v1/gradebook?class_id=${encodeURIComponent(selectedClassId)}`,
      );
    } catch (e) {
      gradebookError = e instanceof Error ? e.message : "Failed to load gradebook";
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Trophy className="size-6" />}
        title="Scores"
        description="View assessments and record scores for your students."
      />

      <section className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h3 className="font-sans font-semibold tracking-tight">Gradebook summary</h3>
          {classes.length > 0 ? (
            <nav aria-label="Gradebook class" className="flex flex-wrap gap-2">
              {classes.map((c) => (
                <Link
                  key={c.id}
                  href={`/teacher/scores?class_id=${c.id}`}
                  className={cn(
                    "rounded-full border border-[var(--border)] px-3 py-1 text-sm",
                    c.id === selectedClassId
                      ? "bg-[var(--primary)] text-[var(--primary-foreground)]"
                      : "bg-[var(--surface)] text-[var(--muted-foreground)] hover:text-[var(--foreground)]",
                  )}
                >
                  {c.name}
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
            {gradebook.subjects.map((s) => (
              <StatCard
                key={s.subject_id}
                label={subjectName.get(s.subject_id) ?? "Subject average"}
                value={formatPercent(s.average)}
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
              (classes.length === 0
                ? "Gradebook summaries will appear here once classes exist."
                : "No gradebook summary is available for this class yet.")
            }
          />
        )}
      </section>

      <TeacherScoresClient assessments={assessments} />
    </div>
  );
}
