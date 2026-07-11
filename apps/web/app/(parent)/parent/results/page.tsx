import { Trophy, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

export interface Assessment {
  id: string;
  name: string;
  type: string;
  subject_name?: string;
  date?: string;
}

export interface Score {
  student_id: string;
  score: number;
}

export interface ResultRow {
  id: string;
  student_id: string;
  assessment: string;
  subject: string;
  score: number;
}

export default async function ParentResultsPage() {
  const client = await createServerClient();
  let assessments: Assessment[] = [];
  try {
    assessments = await client.get<Assessment[]>("/api/v1/assessments");
  } catch {
    assessments = [];
  }

  let rows: ResultRow[] = [];
  if (assessments.length > 0) {
    const scoresByAssessment = await Promise.all(
      assessments.map(async (a) => {
        try {
          const scores = await client.get<Score[]>(`/api/v1/assessments/${a.id}/scores`);
          return { assessment: a, scores };
        } catch {
          return { assessment: a, scores: [] as Score[] };
        }
      }),
    );

    rows = scoresByAssessment.flatMap(({ assessment, scores }) =>
      scores.map((s, i) => ({
        id: `${assessment.id}-${s.student_id}-${i}`,
        student_id: s.student_id,
        assessment: assessment.name,
        subject: assessment.subject_name ?? "—",
        score: s.score,
      })),
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Trophy className="size-6" />}
        title="Results"
        description="Published assessment scores for your children."
      />
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-display font-semibold tracking-tight">Assessment scores</h3>
        <div className="mt-4">
          <DataTable
            caption="Published assessment scores"
            columns={[
              { key: "student_id", header: "Student ID", cell: (r) => r.student_id },
              { key: "assessment", header: "Assessment", cell: (r) => r.assessment },
              { key: "subject", header: "Subject", cell: (r) => r.subject },
              { key: "score", header: "Score", cell: (r) => r.score },
            ]}
            rows={rows}
            keyExtractor={(r) => r.id}
            empty={
              <EmptyState
                icon={<ClipboardList className="size-8" />}
                title="No published scores"
                description="Scores will appear here once assessments are graded and published."
              />
            }
          />
        </div>
      </section>
    </div>
  );
}
