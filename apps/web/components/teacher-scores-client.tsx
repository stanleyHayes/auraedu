"use client";

import * as React from "react";
import { useActionState } from "react";
import { AlertTriangle, ClipboardList, LoaderCircle, Trophy } from "lucide-react";
import { Button, DataTable, EmptyState, type DataTableColumn } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { recordScore, type ActionResult } from "@/lib/actions";
import { loadTeacherClassRosterAction } from "@/lib/teacher-actions";
import type { Assessment } from "@/app/(teacher)/teacher/scores/page";

type Student = OpenAPI.student_v1.components["schemas"]["Student"];

const columns: DataTableColumn<Assessment>[] = [
  { key: "name", header: "Assessment", cell: (assessment) => assessment.name },
  {
    key: "type",
    header: "Type",
    cell: (assessment) => <span className="capitalize">{assessment.type}</span>,
  },
  {
    key: "subject",
    header: "Subject",
    cell: (assessment) => assessment.subject_name ?? "Subject unavailable",
  },
  {
    key: "class",
    header: "Class",
    cell: (assessment) => assessment.class_name ?? "No class assigned",
  },
  {
    key: "date",
    header: "Scheduled",
    cell: (assessment) =>
      assessment.scheduled_at
        ? new Date(assessment.scheduled_at).toLocaleDateString("en-GB")
        : "Not scheduled",
  },
  { key: "maximum", header: "Maximum", cell: (assessment) => assessment.max_score ?? "Not set" },
];

export function TeacherScoresClient({
  assessments,
  assessmentsAvailable,
}: {
  assessments: Assessment[];
  assessmentsAvailable: boolean;
}) {
  const [state, formAction, pending] = useActionState<ActionResult | undefined, FormData>(
    recordScore,
    undefined,
  );
  const formRef = React.useRef<HTMLFormElement>(null);
  const [assessmentId, setAssessmentId] = React.useState(assessments[0]?.id ?? "");
  const [rosters, setRosters] = React.useState<Record<string, Student[] | null>>({});
  const [rosterLoading, setRosterLoading] = React.useState(false);
  const assessment = assessments.find((item) => item.id === assessmentId);
  const classId = assessment?.class_id ?? "";
  const roster = classId ? rosters[classId] : [];

  React.useEffect(() => {
    if (state?.success) formRef.current?.reset();
  }, [state]);

  React.useEffect(() => {
    if (!classId || rosters[classId] !== undefined) return;
    let active = true;
    setRosterLoading(true);
    void loadTeacherClassRosterAction(classId).then((result) => {
      if (!active) return;
      setRosters((current) => ({
        ...current,
        [classId]: result.error ? null : (result.students ?? []),
      }));
      setRosterLoading(false);
    });
    return () => {
      active = false;
    };
  }, [classId, rosters]);

  return (
    <div className="space-y-6">
      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h2 className="font-sans font-semibold tracking-tight">Assessments</h2>
        <div className="mt-4">
          {assessmentsAvailable ? (
            <DataTable
              caption="Assessments available for scoring"
              columns={columns}
              rows={assessments}
              keyExtractor={(assessmentRow) => assessmentRow.id}
              empty={
                <EmptyState
                  icon={<ClipboardList className="size-8" />}
                  title="No assessments"
                  description="Assigned assessments will appear here once they are created."
                />
              }
            />
          ) : (
            <EmptyState
              icon={<AlertTriangle className="size-8" />}
              title="Assessments unavailable"
              description="The assessment service could not be reached. No empty assessment list has been assumed."
            />
          )}
        </div>
      </section>

      <section className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-5">
        <h2 className="font-sans font-semibold tracking-tight">Record a score</h2>
        <p className="mt-1 text-sm text-[var(--muted-foreground)]">
          Choose an assessment, then select a learner from its assigned class register.
        </p>
        <form
          ref={formRef}
          action={formAction}
          className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
        >
          <div className="sm:col-span-2 lg:col-span-1">
            <label htmlFor="assessment_id" className="mb-1.5 block text-sm font-medium">
              Assessment
            </label>
            <select
              id="assessment_id"
              name="assessment_id"
              required
              value={assessmentId}
              onChange={(event) => setAssessmentId(event.target.value)}
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              <option value="">Select assessment</option>
              {assessments.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                  {item.subject_name ? ` · ${item.subject_name}` : ""}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="student_id" className="mb-1.5 block text-sm font-medium">
              Learner
            </label>
            <select
              id="student_id"
              name="student_id"
              required
              disabled={!roster || roster.length === 0 || rosterLoading}
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              <option value="">
                {rosterLoading || roster === undefined
                  ? "Loading assigned register…"
                  : roster === null
                    ? "Register unavailable"
                    : roster.length === 0
                      ? "No active learners"
                      : "Select learner"}
              </option>
              {roster?.map((student) => (
                <option key={student.id} value={student.id}>
                  {student.first_name} {student.last_name} · {student.student_code}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="score" className="mb-1.5 block text-sm font-medium">
              Score{assessment?.max_score ? ` (maximum ${assessment.max_score})` : ""}
            </label>
            <input
              id="score"
              name="score"
              type="number"
              step="0.01"
              min={0}
              max={assessment?.max_score}
              required
              className="h-10 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--background)] px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            />
          </div>
          <div className="flex items-end sm:col-span-2 lg:col-span-3">
            <Button
              type="submit"
              loading={pending}
              loadingLabel="Saving"
              disabled={!assessment || !roster || roster.length === 0 || rosterLoading}
              className="w-full sm:w-auto"
            >
              <Trophy className="size-4" />
              Save score
            </Button>
          </div>
        </form>

        {assessment && !assessment.class_id ? (
          <p className="mt-4 text-sm text-[var(--color-warn)]">
            This assessment has no class assignment, so an authoritative learner register cannot be
            loaded.
          </p>
        ) : null}
        {rosterLoading ? (
          <p className="mt-4 inline-flex items-center gap-2 text-sm text-[var(--muted-foreground)]">
            <LoaderCircle className="size-4 animate-spin" />
            Loading the assigned class register…
          </p>
        ) : null}
        {roster === null ? (
          <p role="alert" className="mt-4 text-sm text-[var(--color-crit)]">
            The assigned class register is unavailable. Score entry is blocked.
          </p>
        ) : null}
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
