"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createTimetableEntryAction,
  updateTimetableEntryAction,
  type AdminTimetableActionResult,
} from "@/lib/admin-timetable-actions";

type TimetableEntry = OpenAPI.academic_v1.components["schemas"]["TimetableEntry"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Term = OpenAPI.academic_v1.components["schemas"]["Term"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

const WEEKDAYS = [
  { value: 1, label: "Monday" },
  { value: 2, label: "Tuesday" },
  { value: 3, label: "Wednesday" },
  { value: 4, label: "Thursday" },
  { value: 5, label: "Friday" },
  { value: 6, label: "Saturday" },
  { value: 7, label: "Sunday" },
] as const;

interface AdminTimetableEntryFormProps {
  mode: "create" | "edit";
  entryId?: string;
  initial?: TimetableEntry;
  classes: AcademicClass[];
  terms: Term[];
  subjects: Subject[];
  teachers: Staff[];
  defaultClassId?: string;
  defaultTermId?: string;
  onSuccess?: () => void;
}

export function AdminTimetableEntryForm({
  mode,
  entryId,
  initial,
  classes,
  terms,
  subjects,
  teachers,
  defaultClassId,
  defaultTermId,
  onSuccess,
}: AdminTimetableEntryFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateTimetableEntryAction.bind(null, entryId!)
    : createTimetableEntryAction;

  const [state, formAction, pending] = React.useActionState<AdminTimetableActionResult, FormData>(
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
          <Label htmlFor="class_id">Class</Label>
          <Select
            id="class_id"
            name="class_id"
            defaultValue={initial?.class_id ?? defaultClassId ?? ""}
            disabled={isEdit}
            required
          >
            <option value="" disabled>
              Select class
            </option>
            {classes.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
          {isEdit ? <input type="hidden" name="class_id" value={initial?.class_id ?? ""} /> : null}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="term_id">Term</Label>
          <Select
            id="term_id"
            name="term_id"
            defaultValue={initial?.term_id ?? defaultTermId ?? ""}
            disabled={isEdit}
            required
          >
            <option value="" disabled>
              Select term
            </option>
            {terms.map((term) => (
              <option key={term.id} value={term.id}>
                {term.name}
              </option>
            ))}
          </Select>
          {isEdit ? <input type="hidden" name="term_id" value={initial?.term_id ?? ""} /> : null}
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
          <Label htmlFor="teacher_id">Teacher</Label>
          <Select id="teacher_id" name="teacher_id" defaultValue={initial?.teacher_id ?? ""}>
            <option value="">Unassigned</option>
            {teachers.map((teacher) => (
              <option key={teacher.id} value={teacher.id}>
                {teacher.first_name} {teacher.last_name}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="weekday">Day</Label>
          <Select id="weekday" name="weekday" defaultValue={initial?.weekday ?? 1} required>
            {WEEKDAYS.map((day) => (
              <option key={day.value} value={day.value}>
                {day.label}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="room">Room</Label>
          <Input
            id="room"
            name="room"
            defaultValue={initial?.room ?? ""}
            placeholder="Block A, Room 3"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="start_time">Start time</Label>
          <Input
            id="start_time"
            name="start_time"
            type="time"
            defaultValue={initial?.start_time ?? ""}
            required
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="end_time">End time</Label>
          <Input
            id="end_time"
            name="end_time"
            type="time"
            defaultValue={initial?.end_time ?? ""}
            required
          />
        </div>

        {isEdit ? (
          <div className="space-y-1.5">
            <Label htmlFor="status">Status</Label>
            <Select id="status" name="status" defaultValue={initial?.status ?? "active"}>
              <option value="active">Active</option>
              <option value="cancelled">Cancelled</option>
            </Select>
          </div>
        ) : null}
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Entry saved." : "Period scheduled."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Scheduling"}>
          {isEdit ? "Save changes" : "Schedule period"}
        </Button>
      </div>
    </form>
  );
}
