"use client";

import * as React from "react";
import { useActionState } from "react";
import { ClipboardList, Trophy } from "lucide-react";
import { Button, DataTable, EmptyState, type DataTableColumn } from "@auraedu/ui";
import { recordScore, type ActionResult } from "@/lib/actions";
import type { Assessment } from "@/app/(teacher)/teacher/scores/page";

const columns: DataTableColumn<Assessment>[] = [
  { key: "name", header: "Assessment", cell: (a) => a.name },
  { key: "type", header: "Type", cell: (a) => <span className="capitalize">{a.type}</span> },
  { key: "subject", header: "Subject", cell: (a) => a.subject_name ?? "—" },
  { key: "date", header: "Date", cell: (a) => a.date ?? "—" },
];

interface TeacherScoresClientProps {
  assessments: Assessment[];
}

export function TeacherScoresClient({ assessments }: TeacherScoresClientProps) {
  const [state, formAction, pending] = useActionState<ActionResult | undefined, FormData>(recordScore, undefined);
  const formRef = React.useRef<HTMLFormElement>(null);

  React.useEffect(() => {
    if (state?.success) {
      formRef.current?.reset();
    }
  }, [state]);

  return (
    <div className="space-y-6">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-display font-semibold tracking-tight">Assessments</h3>
        <div className="mt-4">
          <DataTable
            caption="Assessments available for scoring"
            columns={columns}
            rows={assessments}
            keyExtractor={(a) => a.id}
            empty={
              <EmptyState
                icon={<ClipboardList className="size-8" />}
                title="No assessments"
                description="Assessments will appear here once they are created."
              />
            }
          />
        </div>
      </section>

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h3 className="font-display font-semibold tracking-tight">Record a score</h3>
        <p className="mt-1 text-sm text-[var(--muted-foreground)]">
          Enter a score for a student on an existing assessment.
        </p>

        <form ref={formRef} action={formAction} className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div className="sm:col-span-2 lg:col-span-1">
            <label htmlFor="assessment_id" className="mb-1.5 block text-sm font-medium">
              Assessment
            </label>
            <select
              id="assessment_id"
              name="assessment_id"
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              <option value="">Select assessment</option>
              {assessments.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} {a.subject_name ? `· ${a.subject_name}` : ""}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="student_id" className="mb-1.5 block text-sm font-medium">
              Student ID
            </label>
            <input
              id="student_id"
              name="student_id"
              type="text"
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div>
            <label htmlFor="score" className="mb-1.5 block text-sm font-medium">
              Score
            </label>
            <input
              id="score"
              name="score"
              type="number"
              step="0.01"
              min={0}
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div className="flex items-end sm:col-span-2 lg:col-span-3">
            <Button type="submit" loading={pending} loadingLabel="Saving" className="w-full sm:w-auto">
              <Trophy className="size-4" />
              Save score
            </Button>
          </div>
        </form>

        {state?.error ? (
          <p role="alert" className="mt-4 text-sm text-[var(--color-crit)]">
            {state.error}
          </p>
        ) : null}
        {state?.success ? (
          <p role="status" className="mt-4 text-sm text-[var(--color-ok)]">
            Score recorded.
          </p>
        ) : null}
      </section>
    </div>
  );
}
