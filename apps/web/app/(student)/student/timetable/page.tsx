import type { OpenAPI } from "@auraedu/shared-types";
import { CalendarDays, Clock3, MapPin } from "lucide-react";
import { EmptyState, Reveal } from "@auraedu/ui";
import { createServerClient } from "@/lib/api";

type TimetableEntry = OpenAPI.academic_v1.components["schemas"]["TimetableEntry"];
type TimetableList = OpenAPI.academic_v1.components["schemas"]["TimetableList"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type SubjectList = OpenAPI.academic_v1.components["schemas"]["SubjectList"];

const week = [
  { number: 1, short: "Mon", label: "Monday" },
  { number: 2, short: "Tue", label: "Tuesday" },
  { number: 3, short: "Wed", label: "Wednesday" },
  { number: 4, short: "Thu", label: "Thursday" },
  { number: 5, short: "Fri", label: "Friday" },
  { number: 6, short: "Sat", label: "Saturday" },
  { number: 7, short: "Sun", label: "Sunday" },
] as const;

function displayTime(value: string) {
  const [hour = "0", minute = "00"] = value.split(":");
  const numericHour = Number(hour);
  const period = numericHour >= 12 ? "PM" : "AM";
  const clockHour = numericHour % 12 || 12;
  return `${clockHour}:${minute} ${period}`;
}

export default async function StudentTimetablePage() {
  let entries: TimetableEntry[] = [];
  let subjects: Record<string, Subject> = {};
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [schedule, subjectList] = await Promise.all([
      client.get<TimetableList>("/api/v1/timetable"),
      client.get<SubjectList>("/api/v1/subjects?limit=100"),
    ]);
    entries = schedule.data.filter((entry) => entry.status === "active");
    subjects = Object.fromEntries((subjectList.data ?? []).map((subject) => [subject.id, subject]));
  } catch {
    error = "Your timetable could not be loaded right now. Please try again shortly.";
  }

  if (error) {
    return (
      <EmptyState
        icon={<CalendarDays className="size-8" />}
        title="Timetable unavailable"
        description={error}
      />
    );
  }

  if (entries.length === 0) {
    return (
      <EmptyState
        icon={<CalendarDays className="size-8" />}
        title="No scheduled lessons"
        description="Your school has not published an active timetable for your class yet."
      />
    );
  }

  const today = new Date().getDay() || 7;
  const activeDays = week.filter((day) => entries.some((entry) => entry.weekday === day.number));
  const todayLessons = entries.filter((entry) => entry.weekday === today).length;
  const ordered = [...entries].sort(
    (left, right) =>
      left.weekday - right.weekday || left.start_time.localeCompare(right.start_time),
  );

  return (
    <div className="space-y-7">
      <Reveal>
        <section className="relative overflow-hidden rounded-[var(--radius-lg)] bg-[var(--color-navy)] p-6 text-white sm:p-8">
          <div className="pointer-events-none absolute -right-16 -top-20 size-64 rounded-full bg-[var(--color-forest)]/20 blur-3xl" />
          <div className="relative flex flex-col justify-between gap-6 sm:flex-row sm:items-end">
            <div>
              <p className="font-mono text-xs font-bold uppercase tracking-[0.18em] text-[var(--color-signal)]">
                My learning week
              </p>
              <h1 className="mt-3 font-heading text-3xl font-black tracking-tight sm:text-4xl">
                Your class timetable
              </h1>
              <p className="mt-3 max-w-xl text-sm leading-6 text-slate-300">
                Active lessons for your current class, ordered from the first period to the last.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-white/15 text-center">
              <div className="bg-white/10 px-5 py-3">
                <strong className="block text-2xl">{todayLessons}</strong>
                <span className="text-xs text-slate-300">today</span>
              </div>
              <div className="bg-white/10 px-5 py-3">
                <strong className="block text-2xl">{ordered.length}</strong>
                <span className="text-xs text-slate-300">this week</span>
              </div>
            </div>
          </div>
        </section>
      </Reveal>

      <div className="grid gap-5 xl:grid-cols-2">
        {activeDays.map((day, dayIndex) => {
          const lessons = ordered.filter((entry) => entry.weekday === day.number);
          const isToday = day.number === today;
          return (
            <Reveal key={day.number} delay={dayIndex * 55}>
              <section
                className={`h-full overflow-hidden rounded-[var(--radius-md)] border bg-[var(--surface)] ${
                  isToday
                    ? "border-[var(--color-forest)] shadow-[0_14px_40px_-28px_var(--color-forest)]"
                    : "border-[var(--border)]"
                }`}
                aria-labelledby={`timetable-${day.short}`}
              >
                <header className="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
                  <div className="flex items-center gap-3">
                    <span
                      className={`grid size-10 place-items-center rounded-xl text-xs font-black ${
                        isToday
                          ? "bg-[var(--color-forest)] text-white"
                          : "bg-[var(--muted)] text-[var(--foreground)]"
                      }`}
                    >
                      {day.short}
                    </span>
                    <div>
                      <h2 id={`timetable-${day.short}`} className="font-heading font-bold">
                        {day.label}
                      </h2>
                      <p className="text-xs text-[var(--muted-foreground)]">
                        {lessons.length} lesson{lessons.length === 1 ? "" : "s"}
                      </p>
                    </div>
                  </div>
                  {isToday ? (
                    <span className="rounded-full bg-[var(--color-signal)]/20 px-3 py-1 text-xs font-bold text-[var(--color-forest)]">
                      Today
                    </span>
                  ) : null}
                </header>
                <ol className="divide-y divide-[var(--border)]">
                  {lessons.map((entry) => {
                    const subject = subjects[entry.subject_id];
                    return (
                      <li
                        key={entry.id}
                        className="group grid grid-cols-[5.75rem_1fr] gap-4 px-5 py-4 transition-colors hover:bg-[var(--muted)]/45"
                      >
                        <div className="border-r border-[var(--border)] pr-4">
                          <p className="text-sm font-extrabold tabular-nums">
                            {displayTime(entry.start_time)}
                          </p>
                          <p className="mt-1 text-xs tabular-nums text-[var(--muted-foreground)]">
                            {displayTime(entry.end_time)}
                          </p>
                        </div>
                        <div className="min-w-0">
                          <h3 className="truncate font-bold">
                            {subject?.name ?? "Scheduled lesson"}
                          </h3>
                          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--muted-foreground)]">
                            <span className="inline-flex items-center gap-1.5">
                              <Clock3
                                className="size-3.5 text-[var(--primary)]"
                                aria-hidden="true"
                              />
                              {entry.start_time}–{entry.end_time}
                            </span>
                            <span className="inline-flex items-center gap-1.5">
                              <MapPin
                                className="size-3.5 text-[var(--primary)]"
                                aria-hidden="true"
                              />
                              {entry.room ? `Room ${entry.room}` : "Room to be confirmed"}
                            </span>
                          </div>
                        </div>
                      </li>
                    );
                  })}
                </ol>
              </section>
            </Reveal>
          );
        })}
      </div>
    </div>
  );
}
