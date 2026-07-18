"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createAssignmentAction,
  updateAssignmentAction,
  type TeacherActionResult,
} from "@/lib/teacher-actions";

type Assignment = OpenAPI.assessment_v1.components["schemas"]["Assignment"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface AssignmentFormProps {
  mode: "create" | "edit";
  assignmentId?: string;
  initial?: Assignment;
  subjects: Subject[];
  classes: AcademicClass[];
  years: AcademicYear[];
  onSuccess?: () => void;
}

export function AssignmentForm({
  mode,
  assignmentId,
  initial,
  subjects,
  classes,
  years,
  onSuccess,
}: AssignmentFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateAssignmentAction.bind(null, assignmentId!) : createAssignmentAction;

  const [state, formAction, pending] = React.useActionState<TeacherActionResult, FormData>(
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
            placeholder="Fractions worksheet"
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
            {subjects.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
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
            {years.map((y) => (
              <option key={y.id} value={y.id}>
                {y.name}
                {y.is_current ? " (current)" : ""}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="class_id">Class</Label>
          <Select id="class_id" name="class_id" defaultValue={initial?.class_ids?.[0] ?? ""}>
            <option value="">All classes</option>
            {classes.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="due_date">Due date</Label>
          <Input
            id="due_date"
            name="due_date"
            type="date"
            defaultValue={initial?.due_date ? initial.due_date.slice(0, 10) : ""}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="max_score">Max score</Label>
          <Input
            id="max_score"
            name="max_score"
            type="number"
            min={1}
            step="any"
            defaultValue={initial?.max_score ?? ""}
            required={!isEdit}
            placeholder="100"
          />
        </div>

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="instructions">Instructions</Label>
          <Input
            id="instructions"
            name="instructions"
            defaultValue={initial?.instructions ?? ""}
            placeholder="Answer all questions and show your working."
          />
        </div>
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Assignment saved." : "Assignment created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create assignment"}
        </Button>
      </div>
    </form>
  );
}
