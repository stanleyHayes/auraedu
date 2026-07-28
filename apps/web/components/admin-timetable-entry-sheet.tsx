"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminTimetableEntryForm } from "./admin-timetable-entry-form";

type TimetableEntry = OpenAPI.academic_v1.components["schemas"]["TimetableEntry"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type Term = OpenAPI.academic_v1.components["schemas"]["Term"];
type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];
type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];

interface AdminTimetableEntrySheetProps {
  mode: "create" | "edit";
  initial?: TimetableEntry;
  classes: AcademicClass[];
  terms: Term[];
  subjects: Subject[];
  teachers: Staff[];
  defaultClassId?: string;
  defaultTermId?: string;
}

export function AdminTimetableEntrySheet({
  mode,
  initial,
  classes,
  terms,
  subjects,
  teachers,
  defaultClassId,
  defaultTermId,
}: AdminTimetableEntrySheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit timetable entry</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Schedule period
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
              {isEdit ? "Edit period" : "Schedule a period"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Move the period, reassign the teacher or room, or cancel it."
                : "Assign a subject and teacher to a weekly slot. Overlaps are rejected automatically."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminTimetableEntryForm
              mode={mode}
              entryId={initial?.id}
              initial={initial}
              classes={classes}
              terms={terms}
              subjects={subjects}
              teachers={teachers}
              defaultClassId={defaultClassId}
              defaultTermId={defaultTermId}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
