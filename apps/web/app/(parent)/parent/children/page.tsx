import type { OpenAPI } from "@auraedu/shared-types";
import { Baby, BookOpen, CalendarCheck, GraduationCap, UserPlus } from "lucide-react";
import { EmptyState, PageHeader, Reveal } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

type GuardianChildren = OpenAPI.student_v1.components["schemas"]["GuardianChildren"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type ClassList = OpenAPI.academic_v1.components["schemas"]["ClassList"];

export default async function ParentChildrenPage() {
  let family: GuardianChildren | null = null;
  let classes: Record<string, AcademicClass> = {};
  let error: string | null = null;

  try {
    const client = await createServerClient();
    family = await client.get<GuardianChildren>("/api/v1/guardians/me/children");
    try {
      const list = await client.get<ClassList>("/api/v1/classes?limit=100");
      classes = Object.fromEntries((list.data ?? []).map((item) => [item.id, item]));
    } catch {
      classes = {};
    }
  } catch {
    error = "Your linked learner profiles could not be loaded right now.";
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Baby className="size-6" />}
        title="My Children"
        description="School profiles linked to your authenticated guardian account."
      />
      {error ? (
        <EmptyState
          icon={<UserPlus className="size-8" />}
          title="Profiles unavailable"
          description={error}
        />
      ) : !family || family.students.length === 0 ? (
        <EmptyState
          icon={<UserPlus className="size-8" />}
          title="No children linked yet"
          description="Ask your school to link your guardian account to the correct learner records."
        />
      ) : (
        <section className="grid gap-5 md:grid-cols-2 xl:grid-cols-3" aria-label="Linked children">
          {family.students.map((student, index) => (
            <Reveal key={student.id} delay={index * 55}>
              <article className="h-full overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)]">
                <div className="bg-[var(--color-navy)] p-5 text-white">
                  <div className="flex items-center gap-4">
                    <span className="grid size-12 place-items-center rounded-full bg-white/10 font-heading text-lg font-black">
                      {student.first_name[0]}
                      {student.last_name[0]}
                    </span>
                    <div className="min-w-0">
                      <h2 className="truncate font-heading text-lg font-bold">
                        {student.first_name} {student.last_name}
                      </h2>
                      <p className="mt-1 font-mono text-xs text-slate-300">
                        {student.student_code}
                      </p>
                    </div>
                  </div>
                </div>
                <div className="space-y-4 p-5">
                  <div className="flex items-center justify-between gap-3 text-sm">
                    <span className="inline-flex items-center gap-2 text-[var(--muted-foreground)]">
                      <GraduationCap className="size-4 text-[var(--primary)]" aria-hidden="true" />{" "}
                      Class
                    </span>
                    <strong>
                      {student.class_id
                        ? (classes[student.class_id]?.name ?? "Assigned")
                        : "Not assigned"}
                    </strong>
                  </div>
                  <div className="flex items-center justify-between gap-3 text-sm">
                    <span className="text-[var(--muted-foreground)]">Status</span>
                    <span className="rounded-full bg-[var(--muted)] px-2.5 py-1 text-xs font-bold capitalize">
                      {student.status ?? "active"}
                    </span>
                  </div>
                  <div className="grid grid-cols-2 gap-2 border-t border-[var(--border)] pt-4">
                    <a
                      href="/parent/attendance"
                      className="inline-flex items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-[var(--muted)] px-3 py-2 text-xs font-bold hover:text-[var(--primary)]"
                    >
                      <CalendarCheck className="size-4" aria-hidden="true" /> Attendance
                    </a>
                    <a
                      href="/parent/results"
                      className="inline-flex items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-[var(--muted)] px-3 py-2 text-xs font-bold hover:text-[var(--primary)]"
                    >
                      <BookOpen className="size-4" aria-hidden="true" /> Results
                    </a>
                  </div>
                </div>
              </article>
            </Reveal>
          ))}
        </section>
      )}
    </div>
  );
}
