"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { TeacherCbtExamForm } from "./teacher-cbt-exam-form";
import type { Exam } from "@/lib/teacher-cbt-utils";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface TeacherCbtExamFormSheetProps {
  mode: "create" | "edit";
  initial?: Exam;
  subjects: Subject[];
  years: AcademicYear[];
}

export function TeacherCbtExamFormSheet({
  mode,
  initial,
  subjects,
  years,
}: TeacherCbtExamFormSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {initial?.title}</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Create exam
        </Button>
      )}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">
              {isEdit ? "Edit exam" : "Create exam"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the title, duration, or exam window."
                : "Set up a computer-based exam for one of your subjects. Questions are added next."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <TeacherCbtExamForm
              mode={mode}
              examId={initial?.id}
              initial={initial}
              subjects={subjects}
              years={years}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
