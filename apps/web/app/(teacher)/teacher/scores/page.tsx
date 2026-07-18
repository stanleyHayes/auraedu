import { Trophy } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { TeacherScoresClient } from "@/components/teacher-scores-client";

// subject_name/date are display-only legacy fields not present in the contract.
export type Assessment = OpenAPI.assessment_v1.components["schemas"]["Assessment"] & {
  subject_name?: string;
  date?: string;
};

export default async function TeacherScoresPage() {
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

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Trophy className="size-6" />}
        title="Scores"
        description="View assessments and record scores for your students."
      />
      <TeacherScoresClient assessments={assessments} />
    </div>
  );
}
