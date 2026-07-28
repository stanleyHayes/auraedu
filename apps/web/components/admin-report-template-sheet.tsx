"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminReportTemplateForm } from "./admin-report-template-form";

type ReportTemplate = OpenAPI.report_v1.components["schemas"]["ReportTemplate"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface AdminReportTemplateSheetProps {
  mode: "create" | "edit";
  initial?: ReportTemplate;
  years: AcademicYear[];
}

export function AdminReportTemplateSheet({ mode, initial, years }: AdminReportTemplateSheetProps) {
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
          Add template
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
              {isEdit ? "Edit report template" : "Add report template"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the layout, reassign the academic year, or archive the template."
                : "Define the report-card layout for an academic year before cards are prepared."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminReportTemplateForm
              mode={mode}
              templateId={initial?.id}
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
