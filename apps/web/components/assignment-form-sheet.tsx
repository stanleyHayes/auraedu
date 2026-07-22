"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AssignmentForm } from "./assignment-form";

type Assignment = OpenAPI.assessment_v1.components["schemas"]["Assignment"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface AssignmentFormSheetProps {
  mode: "create" | "edit";
  initial?: Assignment;
  subjects: Subject[];
  classes: AcademicClass[];
  years: AcademicYear[];
}

export function AssignmentFormSheet({
  mode,
  initial,
  subjects,
  classes,
  years,
}: AssignmentFormSheetProps) {
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
          Add assignment
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
              {isEdit ? "Edit assignment" : "Add assignment"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the title, instructions, class, due date, or max score."
                : "Create an assignment for a subject and optionally target a class."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AssignmentForm
              mode={mode}
              assignmentId={initial?.id}
              initial={initial}
              subjects={subjects}
              classes={classes}
              years={years}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
