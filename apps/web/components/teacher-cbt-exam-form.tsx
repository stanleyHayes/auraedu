"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createExamAction,
  updateExamAction,
  type TeacherCbtActionResult,
} from "@/lib/teacher-cbt-actions";
import { toDateTimeLocalValue, type Exam } from "@/lib/teacher-cbt-utils";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface TeacherCbtExamFormProps {
  mode: "create" | "edit";
  examId?: string;
  initial?: Exam;
  subjects: Subject[];
  years: AcademicYear[];
  onSuccess?: () => void;
}

export function TeacherCbtExamForm({
  mode,
  examId,
  initial,
  subjects,
  years,
  onSuccess,
}: TeacherCbtExamFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateExamAction.bind(null, examId!) : createExamAction;

  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    action,
    {},
  );

  React.useEffect(() => {
    if (state.success && onSuccess) {
      onSuccess();
    }
  }, [state, onSuccess]);

  return (
    <form action={formAction} className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="title">Title</Label>
          <Input
            id="title"
            name="title"
            defaultValue={initial?.title}
            required
            placeholder="End-of-term mathematics CBT"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="subject_id">Subject</Label>
          <Select
            id="subject_id"
            name="subject_id"
            defaultValue={initial?.subject_id ?? ""}
            disabled={isEdit}
            required
          >
            <option value="" disabled>
              Select subject
            </option>
            {subjects.map((subject) => (
              <option key={subject.id} value={subject.id}>
                {subject.name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="academic_year_id">Academic year</Label>
          <Select
            id="academic_year_id"
            name="academic_year_id"
            defaultValue={initial?.academic_year_id ?? ""}
            disabled={isEdit}
            required
          >
            <option value="" disabled>
              Select academic year
            </option>
            {years.map((year) => (
              <option key={year.id} value={year.id}>
                {year.name}
                {year.is_current ? " (current)" : ""}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="duration_minutes">Duration (minutes)</Label>
          <Input
            id="duration_minutes"
            name="duration_minutes"
            type="number"
            min={1}
            step={1}
            defaultValue={initial?.duration_minutes ?? ""}
            required
            placeholder="60"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="start_at">Opens at</Label>
          <Input
            id="start_at"
            name="start_at"
            type="datetime-local"
            defaultValue={toDateTimeLocalValue(initial?.start_at)}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="end_at">Closes at</Label>
          <Input
            id="end_at"
            name="end_at"
            type="datetime-local"
            defaultValue={toDateTimeLocalValue(initial?.end_at)}
          />
        </div>
      </div>

      <p className="text-sm text-muted-foreground">
        Questions are added from the exam page after the exam is created.
      </p>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Exam saved." : "Exam created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create exam"}
        </Button>
      </div>
    </form>
  );
}
