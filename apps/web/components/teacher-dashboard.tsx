"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { CalendarDays, CheckSquare, GraduationCap, Megaphone, PenLine } from "lucide-react";
import { Button, StatCard, Reveal, Watermark } from "@auraedu/ui";

interface TeacherDashboardProps {
  userName?: string;
  summary: {
    assignedClasses: number | null;
    classesToday: number | null;
    activeAssignments: number | null;
    draftAssignments: number | null;
    lessons: { id: string; time: string; title: string; room?: string | null }[] | null;
    announcements: { id: string; title: string; body: string }[] | null;
  };
}

export function TeacherDashboard({ userName, summary }: TeacherDashboardProps) {
  const router = useRouter();
  const greeting = userName ? `Welcome back, ${userName}` : "Welcome back";

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Teach
      </Watermark>
      <Reveal>
        <section className="portal-hero card card-hover p-6 sm:p-8">
          <div className="flex items-start gap-3.5">
            <span
              aria-hidden="true"
              className="grid size-12 flex-none place-items-center rounded-[var(--radius-lg)] bg-gradient-to-br from-[var(--portal-accent,var(--color-forest))] to-[var(--color-teal-bright)] text-[var(--color-navy)]"
            >
              <GraduationCap className="size-6" />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="font-heading text-xl font-extrabold tracking-tight">{greeting}</h2>
              <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                Here is what is happening in your classes today.
              </p>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button onClick={() => router.push("/teacher/attendance")}>
              <CheckSquare className="size-4" />
              Mark attendance
            </Button>
            <Button variant="secondary" onClick={() => router.push("/teacher/scores")}>
              <PenLine className="size-4" />
              Record scores
            </Button>
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Classes today" value={summary.classesToday ?? "—"} unit="sessions" />
          <StatCard
            label="Assigned classes"
            value={summary.assignedClasses ?? "—"}
            unit="groups"
            tone="ok"
          />
          <StatCard
            label="Draft assignments"
            value={summary.draftAssignments ?? "—"}
            unit="to review"
            tone="warn"
          />
          <StatCard label="Assignments" value={summary.activeAssignments ?? "—"} unit="published" />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Today&apos;s overview</h3>
            {summary.lessons === null ? (
              <div className="mt-4 flex items-center gap-2 rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-4 text-sm text-[var(--muted-foreground)]">
                <CalendarDays
                  className="size-4 shrink-0 text-[var(--primary)]"
                  aria-hidden="true"
                />
                Today&apos;s timetable is temporarily unavailable.
              </div>
            ) : summary.lessons.length > 0 ? (
              <ol className="mt-4 divide-y divide-[var(--border)]">
                {summary.lessons.map((lesson) => (
                  <li key={lesson.id} className="flex gap-3 py-3 first:pt-0 last:pb-0">
                    <CalendarDays
                      className="mt-0.5 size-4 shrink-0 text-[var(--primary)]"
                      aria-hidden="true"
                    />
                    <span className="min-w-0 flex-1">
                      <strong className="block truncate text-sm">{lesson.title}</strong>
                      <small className="text-[var(--muted-foreground)]">
                        {lesson.time}
                        {lesson.room ? ` · Room ${lesson.room}` : ""}
                      </small>
                    </span>
                  </li>
                ))}
              </ol>
            ) : (
              <button
                type="button"
                onClick={() => router.push("/teacher/classes")}
                className="mt-4 inline-flex items-center gap-2 text-left text-sm text-[var(--muted-foreground)] hover:text-foreground hover:underline"
              >
                <CalendarDays
                  className="size-4 shrink-0 text-[var(--primary)]"
                  aria-hidden="true"
                />
                No active teaching periods are scheduled for today. Review assigned classes.
              </button>
            )}
          </div>
        </Reveal>
        <Reveal delay={160}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Recent notifications</h3>
            {summary.announcements === null ? (
              <div className="mt-4 flex flex-col items-center justify-center rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-6 text-center">
                <Megaphone className="size-6 text-[var(--muted-foreground)]" aria-hidden="true" />
                <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                  Staff announcements are temporarily unavailable.
                </p>
              </div>
            ) : summary.announcements.length > 0 ? (
              <ul className="mt-4 space-y-3">
                {summary.announcements.map((announcement) => (
                  <li
                    key={announcement.id}
                    className="rounded-[var(--radius-sm)] bg-[var(--muted)]/55 p-3"
                  >
                    <p className="text-sm font-semibold">{announcement.title}</p>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-[var(--muted-foreground)]">
                      {announcement.body}
                    </p>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="mt-4 flex flex-col items-center justify-center rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-6 text-center">
                <Megaphone className="size-6 text-[var(--muted-foreground)]" aria-hidden="true" />
                <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                  No current staff announcements.
                </p>
              </div>
            )}
          </div>
        </Reveal>
      </section>
    </div>
  );
}
