"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import {
  BookOpen,
  CalendarDays,
  ClipboardList,
  FileText,
  GraduationCap,
  Sparkles,
} from "lucide-react";
import { Button, StatCard, Reveal, Watermark } from "@auraedu/ui";

interface StudentDashboardProps {
  userName?: string;
  summary: {
    classesToday: number | null;
    activeAssignments: number | null;
    publishedResults: number | null;
    upcomingExams: number | null;
    lessons: { id: string; time: string; title: string; room?: string | null }[] | null;
  };
}

export function StudentDashboard({ userName, summary }: StudentDashboardProps) {
  const router = useRouter();
  const greeting = userName ? `Welcome back, ${userName}` : "Welcome back";

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Learn
      </Watermark>
      <Reveal>
        <section className="portal-hero card card-hover p-6 sm:p-8">
          <div className="flex items-start gap-3.5">
            <span
              aria-hidden="true"
              className="grid size-12 flex-none place-items-center rounded-[var(--radius-lg)] bg-gradient-to-br from-[var(--color-brand)] to-[var(--color-forest)] text-white"
            >
              <GraduationCap className="size-6" />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="font-heading text-xl font-extrabold tracking-tight">{greeting}</h2>
              <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                Here is what is happening in your learning today.
              </p>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button onClick={() => router.push("/student/assignments")}>
              <ClipboardList className="size-4" />
              View assignments
            </Button>
            <Button variant="secondary" onClick={() => router.push("/student/results")}>
              <BookOpen className="size-4" />
              Check results
            </Button>
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Classes today" value={summary.classesToday ?? "—"} unit="sessions" />
          <StatCard
            label="Active assignments"
            value={summary.activeAssignments ?? "—"}
            unit="published"
            tone="warn"
          />
          <StatCard
            label="Results published"
            value={summary.publishedResults ?? "—"}
            unit="scores"
            tone="ok"
          />
          <StatCard label="CBT exams" value={summary.upcomingExams ?? "—"} unit="available" />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Today&apos;s classes</h3>
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
                onClick={() => router.push("/student/timetable")}
                className="mt-4 inline-flex items-center gap-2 text-left text-sm text-[var(--muted-foreground)] hover:text-foreground hover:underline"
              >
                <CalendarDays
                  className="size-4 shrink-0 text-[var(--primary)]"
                  aria-hidden="true"
                />
                No active lessons are scheduled for today. Open the full timetable.
              </button>
            )}
          </div>
        </Reveal>
        <Reveal delay={160}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Quick links</h3>
            <ul className="mt-4 space-y-2 text-sm text-[var(--muted-foreground)]">
              <li className="flex items-center gap-2">
                <ClipboardList className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <a href="/student/assignments" className="hover:text-foreground hover:underline">
                  Pending assignments
                </a>
              </li>
              <li className="flex items-center gap-2">
                <FileText className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <a href="/student/report-card" className="hover:text-foreground hover:underline">
                  View report card
                </a>
              </li>
              <li className="flex items-center gap-2">
                <Sparkles className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <a
                  href="/student/recommendations"
                  className="hover:text-foreground hover:underline"
                >
                  Learning recommendations
                </a>
              </li>
            </ul>
          </div>
        </Reveal>
      </section>
    </div>
  );
}
