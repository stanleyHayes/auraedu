"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createFeeStructureAction,
  updateFeeStructureAction,
  type AdminFeesActionResult,
} from "@/lib/admin-fees-actions";

type FeeStructure = OpenAPI.fees_v1.components["schemas"]["FeeStructure"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];

const RECURRENCES = ["one_time", "termly", "monthly", "annually"] as const;
const TARGETS = ["all_students", "specific_student"] as const;

interface AdminFeesStructureFormProps {
  mode: "create" | "edit";
  structureId?: string;
  initial?: FeeStructure;
  years: AcademicYear[];
  onSuccess?: () => void;
}

export function AdminFeesStructureForm({
  mode,
  structureId,
  initial,
  years,
  onSuccess,
}: AdminFeesStructureFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateFeeStructureAction.bind(null, structureId!)
    : createFeeStructureAction;

  const [state, formAction, pending] = React.useActionState<AdminFeesActionResult, FormData>(
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
        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="name">Structure name</Label>
          <Input
            id="name"
            name="name"
            defaultValue={initial?.name}
            required
            placeholder="Tuition — Form 1"
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

        <div className="grid grid-cols-[1fr_auto] gap-3">
          <div className="space-y-1.5">
            <Label htmlFor="amount">Amount</Label>
            <Input
              id="amount"
              name="amount"
              inputMode="decimal"
              defaultValue={initial ? (initial.amount_cents / 100).toFixed(2) : ""}
              required
              placeholder="1500.00"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="currency">Currency</Label>
            <Input
              id="currency"
              name="currency"
              defaultValue={initial?.currency ?? "GHS"}
              required
              minLength={3}
              maxLength={3}
              className="w-20 uppercase"
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="recurrence">Recurrence</Label>
          <Select id="recurrence" name="recurrence" defaultValue={initial?.recurrence ?? "termly"}>
            {RECURRENCES.map((recurrence) => (
              <option key={recurrence} value={recurrence}>
                {recurrence.replaceAll("_", " ")}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="target">Billing scope</Label>
          <Select id="target" name="target" defaultValue={initial?.target ?? "all_students"}>
            {TARGETS.map((target) => (
              <option key={target} value={target}>
                {target === "all_students" ? "All students" : "Specific student"}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="due_day">Due day of period</Label>
          <Input
            id="due_day"
            name="due_day"
            type="number"
            min={1}
            max={31}
            step={1}
            defaultValue={initial?.due_day ?? ""}
            placeholder="15"
          />
        </div>

        {isEdit ? (
          <div className="space-y-1.5">
            <Label htmlFor="status">Status</Label>
            <Select id="status" name="status" defaultValue={initial?.status ?? "active"}>
              <option value="active">Active</option>
              <option value="archived">Archived</option>
            </Select>
          </div>
        ) : null}

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="description">Description</Label>
          <textarea
            id="description"
            name="description"
            rows={3}
            defaultValue={initial?.description ?? ""}
            className="w-full rounded-xl border border-border bg-background p-3 text-sm"
            placeholder="What this charge covers, shown to finance staff when invoicing."
          />
        </div>
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Fee structure saved." : "Fee structure created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create structure"}
        </Button>
      </div>
    </form>
  );
}
