"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import { issueInvoiceAction, type AdminFeesActionResult } from "@/lib/admin-fees-actions";

type FeeStructure = OpenAPI.fees_v1.components["schemas"]["FeeStructure"];

export interface InvoiceStudentOption {
  id: string;
  name: string;
  classId: string | null;
}

export interface InvoiceClassOption {
  id: string;
  name: string;
}

interface AdminFeesInvoiceFormProps {
  structures: FeeStructure[];
  students: InvoiceStudentOption[];
  classes: InvoiceClassOption[];
  onSuccess?: () => void;
}

export function AdminFeesInvoiceForm({
  structures,
  students,
  classes,
  onSuccess,
}: AdminFeesInvoiceFormProps) {
  const [state, formAction, pending] = React.useActionState<AdminFeesActionResult, FormData>(
    issueInvoiceAction,
    {},
  );
  const [classFilter, setClassFilter] = React.useState("");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());

  React.useEffect(() => {
    if (state.success && onSuccess) {
      onSuccess();
    }
  }, [state, onSuccess]);

  const className = new Map(classes.map((item) => [item.id, item.name]));
  const visible = classFilter ? students.filter((s) => s.classId === classFilter) : students;
  const activeStructures = structures.filter((structure) => structure.status === "active");

  function toggle(id: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function toggleVisible() {
    setSelected((current) => {
      const next = new Set(current);
      const allSelected = visible.length > 0 && visible.every((s) => next.has(s.id));
      for (const student of visible) {
        if (allSelected) {
          next.delete(student.id);
        } else {
          next.add(student.id);
        }
      }
      return next;
    });
  }

  return (
    <form action={formAction} className="space-y-5">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="fee_structure_id">Fee structure</Label>
          <Select id="fee_structure_id" name="fee_structure_id" defaultValue="" required>
            <option value="" disabled>
              Select fee structure
            </option>
            {activeStructures.map((structure) => (
              <option key={structure.id} value={structure.id}>
                {structure.name} — {(structure.amount_cents / 100).toFixed(2)} {structure.currency}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="amount">Amount override</Label>
          <Input id="amount" name="amount" inputMode="decimal" placeholder="Structure amount" />
          <p className="text-xs text-muted-foreground">
            Leave empty to bill the full structure amount.
          </p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="due_date">Due date</Label>
          <Input id="due_date" name="due_date" type="date" />
        </div>

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="notes">Notes</Label>
          <textarea
            id="notes"
            name="notes"
            rows={2}
            className="w-full rounded-xl border border-border bg-background p-3 text-sm"
            placeholder="Optional note stored on the invoice."
          />
        </div>
      </div>

      <fieldset className="space-y-3 rounded-xl border border-border p-4">
        <legend className="px-1 text-sm font-semibold">Students to invoice</legend>
        <div className="flex flex-wrap items-center gap-3">
          <Select
            aria-label="Filter students by class"
            value={classFilter}
            onChange={(event) => setClassFilter(event.target.value)}
            className="max-w-56"
          >
            <option value="">All classes</option>
            {classes.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
          <Button type="button" variant="secondary" onClick={toggleVisible}>
            {visible.length > 0 && visible.every((s) => selected.has(s.id))
              ? "Clear shown"
              : "Select shown"}
          </Button>
          <span className="text-xs font-semibold text-muted-foreground">
            {selected.size} selected
          </span>
        </div>
        {visible.length === 0 ? (
          <p className="text-sm text-muted-foreground">No students match this class filter.</p>
        ) : (
          <ul className="grid max-h-64 gap-1 overflow-y-auto sm:grid-cols-2">
            {visible.map((student) => (
              <li key={student.id}>
                <label className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-muted">
                  <input
                    type="checkbox"
                    name="student_ids"
                    value={student.id}
                    checked={selected.has(student.id)}
                    onChange={() => toggle(student.id)}
                    className="size-4 accent-[var(--primary)]"
                  />
                  <span className="font-medium">{student.name}</span>
                  <span className="text-xs text-muted-foreground">
                    {student.classId ? (className.get(student.classId) ?? "—") : "No class"}
                  </span>
                </label>
              </li>
            ))}
          </ul>
        )}
      </fieldset>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          Invoice issued.
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel="Issuing">
          Issue invoice
        </Button>
      </div>
    </form>
  );
}
