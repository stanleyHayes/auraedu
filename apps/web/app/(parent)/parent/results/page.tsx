import { Trophy, ClipboardList } from "lucide-react";
import { PageHeader, DataTable, EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

// subject_name is a display-only legacy field not present in the contract.
type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"] & {
  subject_name?: string;
};

type Score = OpenAPI.assessment_v1.components["schemas"]["Score"];

export interface ResultRow {
  id: string;
  student_id: string;
  assessment: string;
  subject: string;
  score: number;
}

export default async function ParentResultsPage() {
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

  let rows: ResultRow[] = [];
  if (assessments.length > 0) {
    const scoresByAssessment = await Promise.all(
      assessments.map(async (a) => {
        try {
          // The contract declares no ScoreList schema, so type the envelope inline.
          const res = await client.get<{ data?: Score[]; next_cursor?: string | null }>(
            `/api/v1/assessments/${a.id}/scores`,
          );
          return { assessment: a, scores: res.data ?? [] };
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
        <h3 className="font-sans font-semibold tracking-tight">Assessment scores</h3>
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
