"use client";

import * as React from "react";
import { CalendarPlus, Flag, Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select, Sheet } from "@auraedu/ui";
import {
  createAcademicYearAction,
  createTermAction,
  updateAcademicYearAction,
  updateTermAction,
  type AcademicActionResult,
} from "@/lib/academic-actions";

type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type Term = OpenAPI.academic_v1.components["schemas"]["Term"];

type AcademicCalendarFormSheetProps =
  | { kind: "year"; mode: "create"; initial?: never; years?: never }
  | { kind: "year"; mode: "edit"; initial: AcademicYear; years?: never }
  | { kind: "term"; mode: "create"; initial?: never; years: AcademicYear[] }
  | { kind: "term"; mode: "edit"; initial: Term; years: AcademicYear[] };

export function AcademicCalendarFormSheet(props: AcademicCalendarFormSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = props.mode === "edit";
  const isYear = props.kind === "year";
  const label = isYear ? "academic year" : "term";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {props.initial.name}</span>
        </Button>
      ) : (
        <Button
          type="button"
          variant={isYear ? "primary" : "secondary"}
          onClick={() => setOpen(true)}
          disabled={!isYear && props.years.length === 0}
        >
          {isYear ? <CalendarPlus className="mr-2 size-4" /> : <Plus className="mr-2 size-4" />}
          Add {label}
        </Button>
      )}

      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[var(--muted)] px-6 py-5">
            <span className="absolute -right-8 -top-10 size-32 rounded-full bg-[var(--portal-accent,var(--color-brand))]/10" />
            <div className="relative flex items-start gap-3">
              <span className="grid size-10 place-items-center rounded-xl bg-[var(--portal-accent,var(--color-brand))]/10 text-[var(--portal-accent,var(--color-brand))]">
                {isYear ? <CalendarPlus className="size-5" /> : <Flag className="size-5" />}
              </span>
              <div>
                <h2 className="font-heading text-lg font-bold capitalize">
                  {isEdit ? `Edit ${label}` : `Add ${label}`}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {isYear
                    ? "Define the calendar boundary and which year drives current operations."
                    : "Place a teaching term inside its owning academic year."}
                </p>
              </div>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            {isYear ? (
              <AcademicYearForm
                mode={props.mode}
                initial={props.mode === "edit" ? props.initial : undefined}
                onSuccess={() => setOpen(false)}
              />
            ) : (
              <TermForm
                mode={props.mode}
                initial={props.mode === "edit" ? props.initial : undefined}
                years={props.years}
                onSuccess={() => setOpen(false)}
              />
            )}
          </div>
        </div>
      </Sheet>
    </>
  );
}

function AcademicYearForm({
  mode,
  initial,
  onSuccess,
}: {
  mode: "create" | "edit";
  initial?: AcademicYear;
  onSuccess: () => void;
}) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateAcademicYearAction.bind(null, initial!.id)
    : createAcademicYearAction;
  const [state, formAction, pending] = React.useActionState<AcademicActionResult, FormData>(
    action,
    {},
  );
  React.useEffect(() => {
    if (state.success) onSuccess();
  }, [state.success, onSuccess]);

  return (
    <form action={formAction} className="space-y-6">
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label="Academic year name"
          name="name"
          defaultValue={initial?.name}
          placeholder="2026 / 2027"
        />
        <Field
          label="Calendar code"
          name="code"
          defaultValue={initial?.code}
          placeholder="AY-2026"
          required={isEdit}
          optional={!isEdit}
        />
        <DateField label="Starts" name="start_date" defaultValue={initial?.start_date} />
        <DateField label="Ends" name="end_date" defaultValue={initial?.end_date} />
        {isEdit ? (
          <div className="space-y-2 sm:col-span-2">
            <Label htmlFor="status">Lifecycle status</Label>
            <Select id="status" name="status" defaultValue={initial?.status ?? "active"}>
              <option value="active">Active</option>
              <option value="archived">Archived</option>
            </Select>
          </div>
        ) : null}
        <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-[var(--border)] bg-[var(--muted)]/45 p-4 sm:col-span-2">
          <input
            type="checkbox"
            name="is_current"
            defaultChecked={initial?.is_current}
            className="mt-0.5 size-4 accent-[var(--portal-accent,var(--color-brand))]"
          />
          <span>
            <span className="block text-sm font-semibold">Make this the current academic year</span>
            <span className="mt-1 block text-xs leading-5 text-[var(--muted-foreground)]">
              Current-year changes affect the default calendar context used throughout the school.
            </span>
          </span>
        </label>
      </div>
      <ActionFeedback state={state} />
      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel="Saving">
          {isEdit ? "Save academic year" : "Create academic year"}
        </Button>
      </div>
    </form>
  );
}

function TermForm({
  mode,
  initial,
  years,
  onSuccess,
}: {
  mode: "create" | "edit";
  initial?: Term;
  years: AcademicYear[];
  onSuccess: () => void;
}) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateTermAction.bind(null, initial!.id) : createTermAction;
  const [state, formAction, pending] = React.useActionState<AcademicActionResult, FormData>(
    action,
    {},
  );
  const [yearId, setYearId] = React.useState(initial?.academic_year_id ?? "");
  const selectedYear = years.find((year) => year.id === yearId);
  React.useEffect(() => {
    if (state.success) onSuccess();
  }, [state.success, onSuccess]);

  return (
    <form action={formAction} className="space-y-6">
      <div className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="academic_year_id">Academic year</Label>
          <Select
            id="academic_year_id"
            name="academic_year_id"
            value={yearId}
            onChange={(event) => setYearId(event.target.value)}
            disabled={isEdit}
            required
          >
            <option value="" disabled>
              Select the owning year
            </option>
            {years.map((year) => (
              <option key={year.id} value={year.id}>
                {year.name}
                {year.is_current ? " · current" : ""}
              </option>
            ))}
          </Select>
        </div>
        <Field label="Term name" name="name" defaultValue={initial?.name} placeholder="Term 1" />
        <div className="grid gap-5 sm:grid-cols-2">
          <DateField
            label="Starts"
            name="start_date"
            defaultValue={initial?.start_date}
            min={selectedYear?.start_date}
            max={selectedYear?.end_date}
          />
          <DateField
            label="Ends"
            name="end_date"
            defaultValue={initial?.end_date}
            min={selectedYear?.start_date}
            max={selectedYear?.end_date}
          />
        </div>
        {selectedYear ? (
          <p className="rounded-xl bg-[var(--portal-accent,var(--color-brand))]/8 px-4 py-3 text-xs leading-5 text-[var(--muted-foreground)]">
            Dates must sit within {selectedYear.name}: {selectedYear.start_date} to{" "}
            {selectedYear.end_date}.
          </p>
        ) : null}
      </div>
      <ActionFeedback state={state} />
      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel="Saving">
          {isEdit ? "Save term" : "Create term"}
        </Button>
      </div>
    </form>
  );
}

function Field({
  label,
  name,
  defaultValue,
  placeholder,
  required = true,
  optional = false,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  placeholder: string;
  required?: boolean;
  optional?: boolean;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={name}>
        {label}
        {optional ? (
          <span className="font-normal text-[var(--muted-foreground)]"> · optional</span>
        ) : null}
      </Label>
      <Input
        id={name}
        name={name}
        required={required}
        defaultValue={defaultValue}
        placeholder={placeholder}
      />
    </div>
  );
}

function DateField({
  label,
  name,
  defaultValue,
  min,
  max,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  min?: string;
  max?: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={name}>{label}</Label>
      <Input
        id={name}
        name={name}
        type="date"
        required
        defaultValue={defaultValue}
        min={min}
        max={max}
      />
    </div>
  );
}

function ActionFeedback({ state }: { state: AcademicActionResult }) {
  return state.error ? (
    <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
      {state.error}
    </p>
  ) : null;
}
