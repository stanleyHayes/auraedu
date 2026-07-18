"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { SubjectForm } from "./subject-form";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];

interface SubjectFormSheetProps {
  mode: "create" | "edit";
  initial?: Subject;
}

export function SubjectFormSheet({ mode, initial }: SubjectFormSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button
          type="button"
          variant="ghost"
          className="h-8 px-2"
          onClick={() => setOpen(true)}
        >
          <Pencil className="size-4" />
          <span className="sr-only">Edit {initial?.name}</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Add subject
        </Button>
      )}
      <Sheet open={open} onClose={() => setOpen(false)} side="right" className="w-full max-w-xl bg-[var(--surface)] p-0">
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">{isEdit ? "Edit subject" : "Add subject"}</h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the subject name, code, or description."
                : "Create a subject offered by your school."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <SubjectForm
              mode={mode}
              subjectId={initial?.id}
              initial={initial}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
