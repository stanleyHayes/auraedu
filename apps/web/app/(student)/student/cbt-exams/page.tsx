import { Monitor } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";
import type { components as CbtComponents } from "@auraedu/shared-types/openapi/cbt.v1";

type Exam = CbtComponents["schemas"]["Exam"];

export default async function StudentCbtExamsPage() {
  let exams: Exam[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const list = await client.get<{ data?: Exam[]; next_cursor?: string | null }>("/api/v1/cbt/exams");
    exams = list.data ?? [];
  } catch {
    error = "Could not load CBT exams right now.";
  }

  if (error) {
    return (
      <EmptyState
        icon={<Monitor className="size-8" />}
        title="CBT exams unavailable"
        description={error}
      />
    );
  }

  if (exams.length === 0) {
    return (
      <EmptyState
        icon={<Monitor className="size-8" />}
        title="No upcoming CBT exams"
        description="Your scheduled computer-based tests will appear here."
      />
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="font-display text-lg font-semibold tracking-tight">CBT exams</h2>
      <ul className="space-y-3">
        {exams.map((exam) => (
          <li
            key={exam.id}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="font-medium text-[var(--foreground)]">{exam.title}</h3>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  {exam.question_count ?? 0} questions · {exam.duration_minutes ?? 0} minutes
                </p>
              </div>
              {exam.starts_at ? (
                <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                  Starts {new Date(exam.starts_at).toLocaleString()}
                </span>
              ) : null}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
