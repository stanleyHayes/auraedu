import { ClipboardList, Trophy } from "lucide-react";
import { DataTable, EmptyState, PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"];
type Score = OpenAPI.assessment_v1.components["schemas"]["Score"];
type GuardianChildren = OpenAPI.student_v1.components["schemas"]["GuardianChildren"];

interface ResultRow {
  id: string;
  student: string;
  assessment: string;
  subject: string;
  score: number;
  maximum: number | null;
}

export default async function ParentResultsPage() {
  let rows: ResultRow[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [family, assessmentList] = await Promise.all([
      client.get<GuardianChildren>("/api/v1/guardians/me/children"),
      client.get<OpenAPI.assessment_v1.components["schemas"]["AssessmentList"]>(
        "/api/v1/assessments?status=published&limit=50",
      ),
    ]);
    const assessments = assessmentList.data ?? [];
    const names = new Map(
      family.students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
    );
    let subjectNames = new Map<string, string>();
    try {
      const subjects = await client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
        "/api/v1/subjects?limit=100",
      );
      subjectNames = new Map((subjects.data ?? []).map((subject) => [subject.id, subject.name]));
    } catch {
      subjectNames = new Map();
    }

    const scoresByAssessment = await Promise.all(
      assessments.map(async (assessment: Assessment) => {
        const list = await client.get<OpenAPI.assessment_v1.components["schemas"]["ScoreList"]>(
          `/api/v1/assessments/${encodeURIComponent(assessment.id)}/scores?limit=100`,
        );
        return { assessment, scores: list.data ?? [] };
      }),
    );
    rows = scoresByAssessment.flatMap(({ assessment, scores }) =>
      scores.map((score: Score) => ({
        id: `${assessment.id}-${score.id}`,
        student: names.get(score.student_id) ?? "Linked learner",
        assessment: assessment.name,
        subject: subjectNames.get(assessment.subject_id) ?? "Subject unavailable",
        score: score.score,
        maximum: score.max_score ?? assessment.max_score ?? null,
      })),
    );
  } catch {
    error = "Published results for your linked children could not be loaded right now.";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Trophy className="size-6" />}
        title="Results"
        description="Published assessment scores for your linked children."
      />
      {error ? (
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="Results unavailable"
          description={error}
        />
      ) : (
        <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
          <h2 className="font-sans font-semibold tracking-tight">Assessment scores</h2>
          <div className="mt-4">
            <DataTable
              caption="Published assessment scores"
              columns={[
                {
                  key: "student",
                  header: "Learner",
                  cell: (row) => <span className="font-semibold">{row.student}</span>,
                },
                { key: "assessment", header: "Assessment", cell: (row) => row.assessment },
                { key: "subject", header: "Subject", cell: (row) => row.subject },
                {
                  key: "score",
                  header: "Score",
                  cell: (row) => (
                    <span className="font-mono font-bold tabular-nums">
                      {row.score}
                      {row.maximum === null ? "" : ` / ${row.maximum}`}
                    </span>
                  ),
                },
                {
                  key: "percentage",
                  header: "%",
                  cell: (row) =>
                    row.maximum && row.maximum > 0
                      ? `${Math.round((row.score / row.maximum) * 100)}%`
                      : "—",
                },
              ]}
              rows={rows}
              keyExtractor={(row) => row.id}
              empty={
                <EmptyState
                  icon={<ClipboardList className="size-8" />}
                  title="No published scores"
                  description="Scores will appear after the school publishes an assessment result."
                />
              }
            />
          </div>
        </section>
      )}
    </div>
  );
}
