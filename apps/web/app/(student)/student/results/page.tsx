import { BookOpen } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import type { components as AssessmentComponents } from "@auraedu/shared-types/openapi/assessment.v1";

type Assessment = AssessmentComponents["schemas"]["Assessment"];

export default async function StudentResultsPage() {
  let assessments: Assessment[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const list = await client.get<{ data?: Assessment[]; next_cursor?: string | null }>(
      "/api/v1/assessments",
    );
    assessments = list.data ?? [];
  } catch {
    error = "Could not load results right now.";
  }

  if (error) {
    return (
      <EmptyState
        icon={<BookOpen className="size-8" />}
        title="Results unavailable"
        description={error}
      />
    );
  }

  if (assessments.length === 0) {
    return (
      <EmptyState
        icon={<BookOpen className="size-8" />}
        title="No results yet"
        description="Your published results will appear here."
      />
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="font-heading text-lg font-semibold tracking-tight">Results</h2>
      <ul className="space-y-3">
        {assessments.map((assessment) => (
          <li
            key={assessment.id}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="font-medium text-[var(--foreground)]">{assessment.name}</h3>
                <p className="mt-1 text-sm capitalize text-[var(--muted-foreground)]">
                  {assessment.type}
                </p>
              </div>
              <span className="shrink-0 font-mono text-sm text-[var(--foreground)]">
                — / {assessment.max_score ?? "—"}
              </span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
