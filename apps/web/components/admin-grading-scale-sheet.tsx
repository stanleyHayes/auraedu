"use client";

import * as React from "react";
import { Pencil, Plus, Scale } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminGradingScaleForm } from "./admin-grading-scale-form";

type GradingScale = OpenAPI.academic_v1.components["schemas"]["GradingScale"];

export function AdminGradingScaleSheet({
  mode,
  initial,
}: {
  mode: "create" | "edit";
  initial?: GradingScale;
}) {
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
          <Plus className="size-4" /> New grading scale
        </Button>
      )}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-2xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_88%,var(--portal-accent-soft))] px-6 py-6">
            <span className="absolute -right-10 -top-14 size-36 rounded-full bg-[var(--portal-accent)]/10 blur-2xl" />
            <Scale className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">
              {isEdit ? "Edit grading scale" : "Define a grading scale"}
            </h2>
            <p className="relative mt-1 max-w-lg text-sm leading-6 text-[var(--muted-foreground)]">
              {isEdit
                ? "Adjusting bands changes how future scores are graded; published results keep the grade they were awarded."
                : "One scale per grading policy — assessments and report cards reference it instead of inventing their own ranges."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminGradingScaleForm mode={mode} initial={initial} onSuccess={() => setOpen(false)} />
          </div>
        </div>
      </Sheet>
    </>
  );
}
