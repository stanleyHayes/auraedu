import { CalendarDays } from "lucide-react";
import { Button, EmptyState, PageHeader, Reveal, Select, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminTimetableEntrySheet } from "@/components/admin-timetable-entry-sheet";
import { AdminTimetableDeleteButton } from "@/components/admin-timetable-delete-button";

type TimetableEntry = OpenAPI.academic_v1.components["schemas"]["TimetableEntry"];

const WEEKDAY_LABELS = [
  "",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
  "Sunday",
];

export default async function AdminTimetablePage({
  searchParams,
}: {
  searchParams: Promise<{ class_id?: string; term_id?: string }>;
}) {
  await requireAuth();
  const { class_id: classId = "", term_id: termId = "" } = await searchParams;
  const client = await createServerClient();

  const [classResult, termResult, subjectResult, staffResult] = await Promise.allSettled([
    client.get<OpenAPI.academic_v1.components["schemas"]["ClassList"]>("/api/v1/classes?limit=50"),
    client.get<OpenAPI.academic_v1.components["schemas"]["TermList"]>("/api/v1/terms?limit=50"),
    client.get<OpenAPI.academic_v1.components["schemas"]["SubjectList"]>(
      "/api/v1/subjects?limit=100",
    ),
    client.get<OpenAPI.staff_v1.components["schemas"]["StaffList"]>("/api/v1/staff?limit=100"),
  ]);

  const classes = classResult.status === "fulfilled" ? (classResult.value.data ?? []) : [];
  const terms = termResult.status === "fulfilled" ? (termResult.value.data ?? []) : [];
  const subjects = subjectResult.status === "fulfilled" ? (subjectResult.value.data ?? []) : [];
  const staff = staffResult.status === "fulfilled" ? (staffResult.value.data ?? []) : [];
  const teachers = staff.filter(
    (member) => member.staff_type === "teacher" && member.status === "active",
  );

  const selectedClass = classes.find((item) => item.id === classId);

  let entries: TimetableEntry[] = [];
  let entriesError: string | null = null;
  if (classId) {
    try {
      const query = new URLSearchParams({ class_id: classId, limit: "100" });
      if (termId) query.set("term_id", termId);
      const list = await client.get<OpenAPI.academic_v1.components["schemas"]["TimetableList"]>(
        `/api/v1/timetable?${query.toString()}`,
      );
      entries = list.data ?? [];
    } catch (e) {
      entriesError = e instanceof Error ? e.message : "Failed to load the timetable";
    }
  }

  const subjectName = new Map(subjects.map((subject) => [subject.id, subject.name]));
  const teacherName = new Map(
    staff.map((member) => [member.id, `${member.first_name} ${member.last_name}`]),
  );
  const activeEntries = entries.filter((entry) => entry.status === "active");
  const daysWithEntries = new Set(activeEntries.map((entry) => entry.weekday));
  const visibleWeekdays = [1, 2, 3, 4, 5, 6, 7].filter(
    (day) => day <= 5 || daysWithEntries.has(day),
  );

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<CalendarDays className="size-6" />}
        title="Timetable"
        description="Schedule weekly teaching periods for each class. Overlapping class or teacher slots are rejected."
        action={
          <AdminTimetableEntrySheet
            mode="create"
            classes={classes}
            terms={terms}
            subjects={subjects}
            teachers={teachers}
            defaultClassId={classId || undefined}
            defaultTermId={termId || undefined}
          />
        }
      />

      <form method="get" className="flex flex-wrap items-end gap-3">
        <div className="space-y-1.5">
          <label htmlFor="class_id" className="text-sm font-semibold">
            Class
          </label>
          <Select id="class_id" name="class_id" defaultValue={classId} className="min-w-44">
            <option value="">Choose a class</option>
            {classes.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-1.5">
          <label htmlFor="term_id" className="text-sm font-semibold">
            Term
          </label>
          <Select id="term_id" name="term_id" defaultValue={termId} className="min-w-44">
            <option value="">All terms</option>
            {terms.map((term) => (
              <option key={term.id} value={term.id}>
                {term.name}
              </option>
            ))}
          </Select>
        </div>
        <Button type="submit" variant="secondary">
          View timetable
        </Button>
      </form>

      <section className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Scheduled periods"
          value={classId ? activeEntries.length : "—"}
          unit={selectedClass ? selectedClass.name : "choose a class"}
        />
        <StatCard
          label="Teachers assigned"
          value={
            classId ? new Set(activeEntries.map((e) => e.teacher_id).filter(Boolean)).size : "—"
          }
          unit={`of ${teachers.length} teachers`}
        />
        <StatCard
          label="Days covered"
          value={classId ? daysWithEntries.size : "—"}
          unit="per week"
        />
      </section>

      {!classId ? (
        <EmptyState
          icon={<CalendarDays className="size-8" />}
          title="Choose a class"
          description="Select a class above to view and edit its weekly timetable."
        />
      ) : entriesError ? (
        <EmptyState
          icon={<CalendarDays className="size-8" />}
          title="Could not load the timetable"
          description={entriesError}
        />
      ) : activeEntries.length === 0 ? (
        <EmptyState
          icon={<CalendarDays className="size-8" />}
          title={`No periods scheduled${selectedClass ? ` for ${selectedClass.name}` : ""}`}
          description="Schedule the first period to build the weekly grid."
        />
      ) : (
        <Reveal delay={70}>
          <div
            className="grid gap-3"
            style={{ gridTemplateColumns: `repeat(${visibleWeekdays.length}, minmax(0, 1fr))` }}
          >
            {visibleWeekdays.map((day) => {
              const dayEntries = activeEntries
                .filter((entry) => entry.weekday === day)
                .sort((a, b) => a.start_time.localeCompare(b.start_time));
              return (
                <section
                  key={day}
                  className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-3 shadow-sm"
                >
                  <h2 className="border-b border-border pb-2 text-center font-heading text-sm font-bold">
                    {WEEKDAY_LABELS[day]}
                  </h2>
                  {dayEntries.length === 0 ? (
                    <p className="py-6 text-center text-xs text-muted-foreground">No periods</p>
                  ) : (
                    <ul className="mt-2 space-y-2">
                      {dayEntries.map((entry) => (
                        <li
                          key={entry.id}
                          className="rounded-xl border border-border bg-background/70 p-3"
                        >
                          <p className="font-mono text-xs font-semibold text-[var(--primary)]">
                            {entry.start_time} – {entry.end_time}
                          </p>
                          <p className="mt-1 text-sm font-bold">
                            {subjectName.get(entry.subject_id) ?? "Subject"}
                          </p>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {entry.teacher_id
                              ? (teacherName.get(entry.teacher_id) ?? "Teacher")
                              : "Unassigned"}
                            {entry.room ? ` · ${entry.room}` : ""}
                          </p>
                          <div className="mt-2 flex items-center gap-1">
                            <AdminTimetableEntrySheet
                              mode="edit"
                              initial={entry}
                              classes={classes}
                              terms={terms}
                              subjects={subjects}
                              teachers={teachers}
                            />
                            <AdminTimetableDeleteButton
                              id={entry.id}
                              subjectName={subjectName.get(entry.subject_id) ?? "this period"}
                            />
                          </div>
                        </li>
                      ))}
                    </ul>
                  )}
                </section>
              );
            })}
          </div>
        </Reveal>
      )}

      {classId && entries.some((entry) => entry.status === "cancelled") ? (
        <p className="text-xs text-muted-foreground">
          {entries.filter((entry) => entry.status === "cancelled").length} cancelled period(s) are
          hidden from the grid.
        </p>
      ) : null}
    </div>
  );
}
