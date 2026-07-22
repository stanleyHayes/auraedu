import { CalendarClock, ClipboardList } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
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

  const header = (
    <PageHeader
      eyebrow="Learning workspace"
      icon={<ClipboardList className="size-7" />}
      title="Assignments"
      description="Published classwork, deadlines and the next clear action—kept together for your school week."
    />
  );

  if (error) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="Assignments unavailable"
          description={error}
        />
      </div>
    );
  }

  if (assignments.length === 0) {
    return (
      <div className="space-y-6">
        {header}
        <EmptyState
          icon={<ClipboardList className="size-8" />}
          title="No pending assignments"
          description="You have no assignments due at the moment."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {header}
      <ul className="grid gap-4 md:grid-cols-2" aria-label="Published assignments">
        {assignments.map((assignment, index) => (
          <Reveal key={assignment.id} delay={index * 45}>
            <li className="group relative h-full overflow-hidden rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5 transition-[border-color,box-shadow,transform] duration-300 hover:-translate-y-0.5 hover:border-[var(--color-brand)]/40 hover:shadow-lg">
              <span className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--color-brand)] via-[var(--color-teal-bright)] to-[var(--color-signal)] opacity-90" />
              <div className="flex items-start gap-4">
                <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-[var(--accent)] text-[var(--primary)]">
                  <ClipboardList className="size-5" aria-hidden="true" />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="font-mono text-[10px] font-black uppercase tracking-[0.16em] text-[var(--primary)]">
                    Published work
                  </p>
                  <h2 className="mt-2 text-balance font-heading text-lg font-bold text-[var(--foreground)]">
                    {assignment.title}
                  </h2>
                  <p className="mt-2 truncate text-sm text-[var(--muted-foreground)]">
                    Subject {assignment.subject_id}
                  </p>
                </div>
              </div>
              <div className="mt-5 flex items-center justify-between gap-3 border-t border-[var(--border)] pt-4 text-xs">
                <span className="inline-flex items-center gap-2 font-semibold text-[var(--muted-foreground)]">
                  <CalendarClock className="size-4 text-[var(--primary)]" aria-hidden="true" />
                  {assignment.due_date
                    ? `Due ${new Date(assignment.due_date).toLocaleDateString("en-GB")}`
                    : "No deadline set"}
                </span>
                <span className="rounded-full bg-[var(--muted)] px-3 py-1 font-bold text-[var(--foreground)]">
                  Ready
                </span>
              </div>
            </li>
          </Reveal>
        ))}
      </ul>
    </div>
  );
}
