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
}

export function StudentDashboard({ userName }: StudentDashboardProps) {
  const router = useRouter();
  const greeting = userName ? `Welcome back, ${userName}` : "Welcome back";

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Learn
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
          <StatCard label="Classes today" value="—" unit="sessions" />
          <StatCard label="Pending assignments" value="—" unit="due" tone="warn" />
          <StatCard label="Results published" value="—" unit="records" tone="ok" />
          <StatCard label="CBT exams" value="—" unit="upcoming" />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Today&apos;s classes</h3>
            <ul className="mt-4 space-y-2 text-sm text-[var(--muted-foreground)]">
              <li className="flex items-center gap-2">
                <CalendarDays className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>Timetable integration is coming soon.</span>
              </li>
              <li className="flex items-center gap-2">
                <BookOpen className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>No classes starting in the next hour.</span>
              </li>
            </ul>
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
                <a href="/student/recommendations" className="hover:text-foreground hover:underline">
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
