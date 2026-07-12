"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import {
  Baby,
  CalendarCheck,
  ClipboardList,
  CreditCard,
  FileText,
  GraduationCap,
  Megaphone,
  Users,
} from "lucide-react";
import { Button, StatCard, Reveal, Watermark } from "@auraedu/ui";

interface ParentDashboardProps {
  userName?: string;
}

export function ParentDashboard({ userName }: ParentDashboardProps) {
  const router = useRouter();
  const greeting = userName ? `Welcome back, ${userName}` : "Welcome back";

  return (
    <div className="relative space-y-8">
      <Watermark className="pointer-events-none absolute -right-8 -top-12 text-[10rem] opacity-[0.03]">
        Family
      </Watermark>
      <Reveal>
        <section className="card card-hover rounded-[var(--radius-md)] p-6">
          <div className="flex items-start gap-3.5">
            <span
              aria-hidden="true"
              className="grid size-12 flex-none place-items-center rounded-[var(--radius-lg)] bg-gradient-to-br from-[var(--color-brand)] to-[var(--color-burgundy)] text-white"
            >
              <Users className="size-6" />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="font-heading text-xl font-extrabold tracking-tight">{greeting}</h2>
              <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                Here is an overview of your children and what is happening at school.
              </p>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button onClick={() => router.push("/parent/children")}>
              <Baby className="size-4" />
              My children
            </Button>
            <Button variant="secondary" onClick={() => router.push("/parent/fees")}>
              <CreditCard className="size-4" />
              View fees
            </Button>
          </div>
        </section>
      </Reveal>

      <Reveal delay={80}>
        <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Children" value="—" unit="enrolled" />
          <StatCard label="Attendance" value="—" unit="this week" tone="ok" />
          <StatCard label="Outstanding fees" value="—" unit="due" tone="warn" />
          <StatCard label="Results" value="—" unit="published" />
        </section>
      </Reveal>

      <section className="grid gap-6 md:grid-cols-2">
        <Reveal delay={120}>
          <div className="card card-hover h-full rounded-[var(--radius-md)] p-5">
            <h3 className="font-sans font-semibold tracking-tight">Children overview</h3>
            <ul className="mt-4 space-y-2 text-sm text-[var(--muted-foreground)]">
              <li className="flex items-center gap-2">
                <GraduationCap className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>Children profiles and class information will appear here.</span>
              </li>
              <li className="flex items-center gap-2">
                <CalendarCheck className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <span>Recent attendance records will be shown for each child.</span>
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
                <a href="/parent/results" className="hover:text-foreground hover:underline">
                  Check latest results
                </a>
              </li>
              <li className="flex items-center gap-2">
                <FileText className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <a href="/parent/reports" className="hover:text-foreground hover:underline">
                  View report cards
                </a>
              </li>
              <li className="flex items-center gap-2">
                <Megaphone className="size-4 text-[var(--primary)]" aria-hidden="true" />
                <a href="/parent/notifications" className="hover:text-foreground hover:underline">
                  Read school notices
                </a>
              </li>
            </ul>
          </div>
        </Reveal>
      </section>
    </div>
  );
}
