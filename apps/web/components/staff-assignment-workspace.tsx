"use client";

import * as React from "react";
import { BookOpen, Layers3, Plus, Sparkles, Trash2, UsersRound } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createStaffAssignmentAction,
  deleteStaffAssignmentAction,
  type StaffActionResult,
} from "@/lib/staff-actions";

type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];
type Assignment = OpenAPI.staff_v1.components["schemas"]["StaffAssignment"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];

export interface StaffAssignmentWorkspaceProps {
  teachers: Staff[];
  assignments: Assignment[];
  classes: AcademicClass[];
  subjects: Subject[];
}

export function StaffAssignmentWorkspace({
  teachers,
  assignments,
  classes,
  subjects,
}: StaffAssignmentWorkspaceProps) {
  const [selectedStaffId, setSelectedStaffId] = React.useState(teachers[0]?.id ?? "");
  const selected = teachers.find((teacher) => teacher.id === selectedStaffId);
  const rows = assignments.filter((assignment) => assignment.staff_id === selectedStaffId);
  const classNames = new Map(classes.map((item) => [item.id, item.name]));
  const subjectNames = new Map(subjects.map((item) => [item.id, item.name]));

  if (teachers.length === 0) {
    return (
      <section className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-8 shadow-[var(--shadow-sm)]">
        <div className="max-w-xl">
          <span className="inline-flex size-11 items-center justify-center rounded-2xl bg-[var(--portal-accent-soft)] text-[var(--portal-accent)]">
            <UsersRound className="size-5" />
          </span>
          <h2 className="mt-5 text-xl font-black tracking-tight">Build your teaching team first</h2>
          <p className="mt-2 text-sm leading-6 text-[var(--muted-foreground)]">
            Create an active teacher record, then return here to connect classes and subjects to
            their secure portal scope.
          </p>
        </div>
      </section>
    );
  }

  return (
    <section className="relative overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] shadow-[var(--shadow-md)]">
      <div className="pointer-events-none absolute right-0 top-0 h-56 w-56 rounded-full bg-[var(--portal-accent)]/10 blur-3xl" />
      <div className="relative grid lg:grid-cols-[0.78fr_1.22fr]">
        <div className="border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_90%,var(--portal-accent-soft))] p-6 lg:border-b-0 lg:border-r">
          <div className="flex items-center gap-3">
            <span className="inline-flex size-10 items-center justify-center rounded-2xl bg-[var(--portal-accent)] text-white shadow-lg shadow-[var(--portal-accent)]/20">
              <Sparkles className="size-4" />
            </span>
            <div>
              <p className="text-[11px] font-black uppercase tracking-[0.2em] text-[var(--portal-accent)]">
                Teaching scope
              </p>
              <h2 className="text-lg font-black tracking-tight">Class & subject map</h2>
            </div>
          </div>

          <Label htmlFor="assignment-teacher" className="mt-7 block">
            Choose a teacher
          </Label>
          <Select
            id="assignment-teacher"
            className="mt-2"
            value={selectedStaffId}
            onChange={(event) => setSelectedStaffId(event.target.value)}
          >
            {teachers.map((teacher) => (
              <option key={teacher.id} value={teacher.id}>
                {teacher.first_name} {teacher.last_name}
              </option>
            ))}
          </Select>

          <div className="mt-5 rounded-2xl border border-[var(--border)] bg-[var(--background)]/75 p-4">
            <p className="font-bold">
              {selected?.first_name} {selected?.last_name}
            </p>
            <p className="mt-1 text-xs text-[var(--muted-foreground)]">
              {selected?.staff_code ?? "Teacher"} · {rows.length} active{" "}
              {rows.length === 1 ? "assignment" : "assignments"}
            </p>
          </div>

          <div className="mt-5 space-y-2">
            {rows.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-[var(--border)] p-5 text-sm text-[var(--muted-foreground)]">
                No explicit scope yet. Add the first class assignment.
              </div>
            ) : (
              rows.map((assignment) => (
                <AssignmentRow
                  key={assignment.id}
                  assignment={assignment}
                  className={classNames.get(assignment.class_id)}
                  subjectName={
                    assignment.subject_id ? subjectNames.get(assignment.subject_id) : undefined
                  }
                />
              ))
            )}
          </div>
        </div>

        <AssignmentForm staffId={selectedStaffId} classes={classes} subjects={subjects} />
      </div>
    </section>
  );
}

function AssignmentForm({
  staffId,
  classes,
  subjects,
}: {
  staffId: string;
  classes: AcademicClass[];
  subjects: Subject[];
}) {
  const action = createStaffAssignmentAction.bind(null, staffId);
  const [state, formAction, pending] = React.useActionState<StaffActionResult, FormData>(
    action,
    {},
  );
  return (
    <form action={formAction} className="relative p-6 sm:p-8">
      <p className="text-[11px] font-black uppercase tracking-[0.2em] text-[var(--muted-foreground)]">
        New assignment
      </p>
      <h3 className="mt-2 text-2xl font-black tracking-tight">Give the right access, once.</h3>
      <p className="mt-2 max-w-xl text-sm leading-6 text-[var(--muted-foreground)]">
        This relationship controls the teacher’s class roster, attendance, assessments, reports and
        mobile workspace.
      </p>

      <div className="mt-7 grid gap-5 sm:grid-cols-2">
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="class_id">Class</Label>
          <Select id="class_id" name="class_id" required defaultValue="">
            <option value="" disabled>
              Select a class
            </option>
            {classes.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="subject_id">
            Subject <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
          </Label>
          <Select id="subject_id" name="subject_id" defaultValue="">
            <option value="">All subjects in class</option>
            {subjects.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="role">
            Responsibility{" "}
            <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
          </Label>
          <Input id="role" name="role" placeholder="e.g. Mathematics teacher" maxLength={100} />
        </div>
      </div>

      {classes.length === 0 ? (
        <p className="mt-5 rounded-xl bg-amber-500/10 px-4 py-3 text-sm text-amber-700">
          Create a class before assigning teachers.
        </p>
      ) : null}
      {state.error ? (
        <p role="alert" className="mt-5 rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p
          role="status"
          className="mt-5 rounded-xl bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700"
        >
          Assignment saved. The teacher’s portal scope is updated.
        </p>
      ) : null}

      <Button
        type="submit"
        className="mt-7"
        disabled={!staffId || classes.length === 0}
        loading={pending}
        loadingLabel="Assigning"
      >
        <Plus className="size-4" /> Assign teacher
      </Button>
    </form>
  );
}

function AssignmentRow({
  assignment,
  className,
  subjectName,
}: {
  assignment: Assignment;
  className?: string;
  subjectName?: string;
}) {
  const [pending, startTransition] = React.useTransition();
  return (
    <div className="group flex items-center gap-3 rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-3 transition hover:-translate-y-0.5 hover:shadow-sm">
      <span className="inline-flex size-9 shrink-0 items-center justify-center rounded-xl bg-[var(--portal-accent-soft)] text-[var(--portal-accent)]">
        {subjectName ? <BookOpen className="size-4" /> : <Layers3 className="size-4" />}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-bold">{className ?? "Class unavailable"}</p>
        <p className="truncate text-xs text-[var(--muted-foreground)]">
          {subjectName ?? "All subjects"}
          {assignment.role ? ` · ${assignment.role}` : ""}
        </p>
      </div>
      <Button
        type="button"
        variant="ghost"
        className="size-9 px-0 text-[var(--muted-foreground)] hover:text-red-600"
        loading={pending}
        loadingLabel="Removing"
        aria-label={`Remove assignment for ${className ?? "class"}`}
        onClick={() =>
          startTransition(
            () => void deleteStaffAssignmentAction(assignment.staff_id, assignment.id),
          )
        }
      >
        <Trash2 className="size-4" />
      </Button>
    </div>
  );
}
