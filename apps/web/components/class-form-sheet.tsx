"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { ClassForm } from "./class-form";

type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

interface ClassFormSheetProps {
  mode: "create" | "edit";
  initial?: AcademicClass;
  years: AcademicYear[];
  staff: Staff[];
}

export function ClassFormSheet({ mode, initial, years, staff }: ClassFormSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {initial?.name}</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Add class
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
              {isEdit ? "Edit class" : "Add class"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the class name, teacher, or capacity."
                : "Create a class for an academic year and optionally assign a class teacher."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <ClassForm
              mode={mode}
              classId={initial?.id}
              initial={initial}
              years={years}
              staff={staff}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
