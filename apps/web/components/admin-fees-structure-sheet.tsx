"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminFeesStructureForm } from "./admin-fees-structure-form";

type FeeStructure = OpenAPI.fees_v1.components["schemas"]["FeeStructure"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface AdminFeesStructureSheetProps {
  mode: "create" | "edit";
  initial?: FeeStructure;
  years: AcademicYear[];
}

export function AdminFeesStructureSheet({ mode, initial, years }: AdminFeesStructureSheetProps) {
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
          Add fee structure
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
              {isEdit ? "Edit fee structure" : "Add fee structure"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the amount, recurrence, billing scope, or archive the structure."
                : "Define a billable charge for an academic year, then issue invoices from it."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminFeesStructureForm
              mode={mode}
              structureId={initial?.id}
              initial={initial}
              years={years}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
