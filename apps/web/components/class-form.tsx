"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createClassAction,
  updateClassAction,
  type AcademicActionResult,
} from "@/lib/academic-actions";

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

interface ClassFormProps {
  mode: "create" | "edit";
  classId?: string;
  initial?: AcademicClass;
  years: AcademicYear[];
  staff: Staff[];
  onSuccess?: () => void;
}

export function ClassForm({ mode, classId, initial, years, staff, onSuccess }: ClassFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateClassAction.bind(null, classId!) : createClassAction;

  const [state, formAction, pending] = React.useActionState<AcademicActionResult, FormData>(
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
        <div className="space-y-1.5">
          <Label htmlFor="name">Class name</Label>
          <Input
            id="name"
            name="name"
            defaultValue={initial?.name}
            required
            placeholder="Form 1A"
          />
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
          <Label htmlFor="class_teacher_id">Class teacher</Label>
          <Select
            id="class_teacher_id"
            name="class_teacher_id"
            defaultValue={initial?.class_teacher_id ?? ""}
          >
            <option value="">None</option>
            {staff.map((s) => (
              <option key={s.id} value={s.id}>
                {s.first_name} {s.last_name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="capacity">Capacity</Label>
          <Input
            id="capacity"
            name="capacity"
            type="number"
            min={1}
            step={1}
            defaultValue={initial?.capacity ?? ""}
            placeholder="40"
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
          {isEdit ? "Class saved." : "Class created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create class"}
        </Button>
      </div>
    </form>
  );
}
