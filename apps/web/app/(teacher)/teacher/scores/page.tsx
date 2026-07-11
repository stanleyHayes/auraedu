import { Trophy } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import { TeacherScoresClient } from "@/components/teacher-scores-client";

export interface Assessment {
  id: string;
  name: string;
  type: string;
  subject_name?: string;
  date?: string;
}

export default async function TeacherScoresPage() {
  const client = await createServerClient();
  let assessments: Assessment[] = [];
  try {
    assessments = await client.get<Assessment[]>("/api/v1/assessments");
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
