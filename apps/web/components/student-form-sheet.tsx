"use client";

import * as React from "react";
import { Pencil, Plus, UserRoundPlus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { StudentForm } from "./student-form";

type Student = OpenAPI.student_v1.components["schemas"]["Student"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type User = OpenAPI.identity_v1.components["schemas"]["User"];

export function StudentFormSheet({
  mode,
  initial,
  classes,
  years,
  users,
}: {
  mode: "create" | "edit";
  initial?: Student;
  classes: AcademicClass[];
  years: AcademicYear[];
  users: User[];
}) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";
  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">
            Edit {initial?.first_name} {initial?.last_name}
          </span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="size-4" /> Add student
        </Button>
      )}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-2xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_88%,var(--portal-accent-soft))] px-6 py-6">
            <span className="absolute -right-10 -top-14 size-36 rounded-full bg-[var(--portal-accent)]/10 blur-2xl" />
            <UserRoundPlus className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">
              {isEdit ? "Edit learner record" : "Welcome a learner"}
            </h2>
            <p className="relative mt-1 max-w-lg text-sm leading-6 text-[var(--muted-foreground)]">
              {isEdit
                ? "Keep the learner’s name and lifecycle status accurate."
                : "Create the learner, optionally establish the first enrolment, and link secure portal access."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <StudentForm
              mode={mode}
              initial={initial}
              classes={classes}
              years={years}
              users={users}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
