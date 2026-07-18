import { ClipboardList } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";

type Assignment = OpenAPI.assessment_v1.components["schemas"]["Assignment"];

export default async function StudentAssignmentsPage() {
  let assignments: Assignment[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const list = await client.get<OpenAPI.assessment_v1.components["schemas"]["AssignmentList"]>(
      "/api/v1/assignments?status=published",
    );
    assignments = list.data ?? [];
  } catch {
    error = "Could not load assignments right now.";
  }

  if (error) {
    return (
      <EmptyState
        icon={<ClipboardList className="size-8" />}
        title="Assignments unavailable"
        description={error}
      />
    );
  }

  if (assignments.length === 0) {
    return (
      <EmptyState
        icon={<ClipboardList className="size-8" />}
        title="No pending assignments"
        description="You have no assignments due at the moment."
      />
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="font-heading text-lg font-semibold tracking-tight">Assignments</h2>
      <ul className="space-y-3">
        {assignments.map((assignment) => (
          <li
            key={assignment.id}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="font-medium text-[var(--foreground)]">{assignment.title}</h3>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  Subject ID: {assignment.subject_id}
                </p>
              </div>
              {assignment.due_date ? (
                <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                  Due {new Date(assignment.due_date).toLocaleDateString()}
                </span>
              ) : null}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
