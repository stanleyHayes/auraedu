"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label } from "@auraedu/ui";
import {
  createSubjectAction,
  updateSubjectAction,
  type AcademicActionResult,
} from "@/lib/academic-actions";

type Subject = OpenAPI.academic_v1.components["schemas"]["Subject"];

interface SubjectFormProps {
  mode: "create" | "edit";
  subjectId?: string;
  initial?: Subject;
  onSuccess?: () => void;
}

export function SubjectForm({ mode, subjectId, initial, onSuccess }: SubjectFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateSubjectAction.bind(null, subjectId!) : createSubjectAction;

  const [state, formAction, pending] = React.useActionState<AcademicActionResult, FormData>(
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
          <Label htmlFor="name">Subject name</Label>
          <Input
            id="name"
            name="name"
            defaultValue={initial?.name}
            required
            placeholder="Mathematics"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="code">Code</Label>
          <Input id="code" name="code" defaultValue={initial?.code ?? ""} placeholder="MATH" />
        </div>

        <div className="space-y-1.5 sm:col-span-2">
          <Label htmlFor="description">Description</Label>
          <Input
            id="description"
            name="description"
            defaultValue={initial?.description ?? ""}
            placeholder="Core mathematics curriculum"
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
          {isEdit ? "Subject saved." : "Subject created."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save changes" : "Create subject"}
        </Button>
      </div>
    </form>
  );
}
