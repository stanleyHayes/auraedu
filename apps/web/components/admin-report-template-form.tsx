"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createReportTemplateAction,
  updateReportTemplateAction,
  type AdminReportActionResult,
} from "@/lib/admin-report-actions";

type ReportTemplate = OpenAPI.report_v1.components["schemas"]["ReportTemplate"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

interface AdminReportTemplateFormProps {
  mode: "create" | "edit";
  templateId?: string;
  initial?: ReportTemplate;
  years: AcademicYear[];
  onSuccess?: () => void;
}

export function AdminReportTemplateForm({
  mode,
  templateId,
  initial,
  years,
  onSuccess,
}: AdminReportTemplateFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateReportTemplateAction.bind(null, templateId!)
    : createReportTemplateAction;

  const [state, formAction, pending] = React.useActionState<AdminReportActionResult, FormData>(
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
          <Label htmlFor="name">Template name</Label>
          <Input
            id="name"
            name="name"
            defaultValue={initial?.name}
            required
            placeholder="End-of-term report"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="academic_year_id">Academic year</Label>
          <Select
            id="academic_year_id"
            name="academic_year_id"
            defaultValue={initial?.academic_year_id ?? ""}
            required
          >
            <option value="" disabled>
              Select academic year
            </option>
            {years.map((year) => (
              <option key={year.id} value={year.id}>
                {year.name}
                {year.is_current ? " (current)" : ""}
              </option>
            ))}
          </Select>
        </div>

        {isEdit ? (
          <div className="space-y-1.5">
            <Label htmlFor="status">Status</Label>
            <Select id="status" name="status" defaultValue={initial?.status ?? "draft"}>
              <option value="draft">Draft</option>
              <option value="active">Active</option>
              <option value="archived">Archived</option>
            </Select>
          </div>
        ) : null}

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="body_template">Body template</Label>
          <textarea
            id="body_template"
            name="body_template"
            rows={8}
            required
            defaultValue={initial?.body_template ?? ""}
            className="w-full rounded-xl border border-border bg-background p-3 font-mono text-sm"
            placeholder="Report layout: learner details, subject scores, attendance summary and remarks."
          />
          <p className="text-xs text-muted-foreground">
            Defines the sections rendered into every report card generated from this template.
          </p>
        </div>
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Template saved." : "Template created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create template"}
        </Button>
      </div>
    </form>
  );
}
