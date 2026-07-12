"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import {
  CalendarDays,
  CheckSquare,
  ClipboardList,
  GraduationCap,
  Megaphone,
  PenLine,
} from "lucide-react";
import { Button, StatCard, Reveal, Watermark } from "@auraedu/ui";

interface TeacherDashboardProps {
  userName?: string;
}

export function TeacherDashboard({ userName }: TeacherDashboardProps) {
  const router = useRouter();
  const greeting = userName ? `Welcome back, ${userName}` : "Welcome back";

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Teach
      </Watermark>
      <Reveal>
        <section className="card card-hover rounded-[var(--radius-md)] p-6">
          <div className="flex items-start gap-3.5">
            <span
              aria-hidden="true"
              className="grid size-12 flex-none place-items-center rounded-[var(--radius-lg)] bg-gradient-to-br from-[var(--color-brand)] to-[var(--color-burgundy)] text-white"
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
          <StatCard label="Classes today" value="—" unit="sessions" />
          <StatCard label="Attendance" value="—" unit="today" tone="ok" />
          <StatCard label="Scores pending" value="—" unit="records" tone="warn" />
          <StatCard label="Assignments" value="—" unit="active" />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Today&apos;s overview</h3>
            <ul className="mt-4 space-y-2 text-sm text-[var(--muted-foreground)]">
              <li className="flex items-center gap-2">
                <CalendarDays className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>Timetable integration is coming soon.</span>
              </li>
              <li className="flex items-center gap-2">
                <ClipboardList className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>No assignments due today.</span>
              </li>
            </ul>
          </div>
        </Reveal>
        <Reveal delay={160}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Recent notifications</h3>
            <div className="mt-4 flex flex-col items-center justify-center rounded-[var(--radius-sm)] border border-dashed border-[var(--border)] p-6 text-center">
              <Megaphone className="size-6 text-[var(--muted-foreground)]" aria-hidden="true" />
              <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                No new notifications. School announcements will appear here.
              </p>
            </div>
          </div>
        </Reveal>
      </section>
    </div>
  );
}
