"use client";

import * as React from "react";
import { Plus, Trash2 } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label } from "@auraedu/ui";
import {
  createGradingScaleAction,
  updateGradingScaleAction,
  type AdminGradingActionResult,
} from "@/lib/admin-grading-actions";

type GradingScale = OpenAPI.academic_v1.components["schemas"]["GradingScale"];
type GradeRange = OpenAPI.academic_v1.components["schemas"]["GradeRange"];

interface BandDraft {
  grade: string;
  min: string;
  max: string;
  remark: string;
}

const emptyBand: BandDraft = { grade: "", min: "", max: "", remark: "" };

function toDrafts(scale?: GradingScale): BandDraft[] {
  const ranges = (scale?.ranges ?? []) as GradeRange[];
  if (ranges.length === 0) {
    return [
      { grade: "A", min: "80", max: "100", remark: "Excellent" },
      { grade: "B", min: "70", max: "79", remark: "Very good" },
      { grade: "C", min: "60", max: "69", remark: "Good" },
    ];
  }
  return ranges.map((range) => ({
    grade: range.grade,
    min: String(range.min),
    max: String(range.max),
    remark: range.remark ?? "",
  }));
}

export function AdminGradingScaleForm({
  mode,
  initial,
  onSuccess,
}: {
  mode: "create" | "edit";
  initial?: GradingScale;
  onSuccess?: () => void;
}) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateGradingScaleAction.bind(null, initial!.id)
    : createGradingScaleAction;
  const [state, formAction, pending] = React.useActionState<AdminGradingActionResult, FormData>(
    action,
    {},
  );
  const [bands, setBands] = React.useState<BandDraft[]>(() => toDrafts(initial));
  React.useEffect(() => {
    if (state.success) onSuccess?.();
  }, [state.success, onSuccess]);

  function updateBand(index: number, patch: Partial<BandDraft>) {
    setBands((current) =>
      current.map((band, position) => (position === index ? { ...band, ...patch } : band)),
    );
  }

  function removeBand(index: number) {
    setBands((current) => (current.length > 1 ? current.filter((_, i) => i !== index) : current));
  }

  return (
    <form action={formAction} className="space-y-6">
      <div className="space-y-2">
        <Label htmlFor="name">Scale name</Label>
        <Input
          id="name"
          name="name"
          required
          maxLength={120}
          defaultValue={initial?.name}
          placeholder="WASSCE letter grades"
        />
      </div>

      <div className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <Label>Grade bands</Label>
          <Button
            type="button"
            variant="secondary"
            className="h-8 gap-1.5 px-2.5 text-xs"
            onClick={() => setBands((current) => [...current, { ...emptyBand }])}
          >
            <Plus className="size-3.5" /> Add band
          </Button>
        </div>
        <p className="text-xs leading-5 text-[var(--muted-foreground)]">
          Bands must stay within 0–100, min may not exceed max, and bands may not overlap — the
          academic service rejects overlapping ranges, and scores are always graded against these
          boundaries.
        </p>
        <div className="space-y-3">
          {bands.map((band, index) => (
            <div
              key={index}
              className="grid grid-cols-[1fr_5rem_5rem_1fr_auto] items-end gap-2 rounded-xl border border-border bg-background/60 p-3"
            >
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                Grade
                <Input
                  aria-label={`Band ${index + 1} grade`}
                  name="band_grade"
                  required
                  maxLength={10}
                  value={band.grade}
                  onChange={(event) => updateBand(index, { grade: event.target.value })}
                  placeholder="A"
                />
              </label>
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                Min
                <Input
                  aria-label={`Band ${index + 1} minimum score`}
                  name="band_min"
                  type="number"
                  required
                  min={0}
                  max={100}
                  step="any"
                  value={band.min}
                  onChange={(event) => updateBand(index, { min: event.target.value })}
                />
              </label>
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                Max
                <Input
                  aria-label={`Band ${index + 1} maximum score`}
                  name="band_max"
                  type="number"
                  required
                  min={0}
                  max={100}
                  step="any"
                  value={band.max}
                  onChange={(event) => updateBand(index, { max: event.target.value })}
                />
              </label>
              <label className="space-y-1 text-xs font-semibold text-muted-foreground">
                Remark <span className="font-normal">(optional)</span>
                <Input
                  aria-label={`Band ${index + 1} remark`}
                  name="band_remark"
                  maxLength={80}
                  value={band.remark}
                  onChange={(event) => updateBand(index, { remark: event.target.value })}
                  placeholder="Excellent"
                />
              </label>
              <Button
                type="button"
                variant="ghost"
                className="h-9 px-2 text-destructive hover:bg-destructive/10"
                disabled={bands.length <= 1}
                onClick={() => removeBand(index)}
              >
                <Trash2 className="size-4" />
                <span className="sr-only">Remove band {index + 1}</span>
              </Button>
            </div>
          ))}
        </div>
      </div>

      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}
      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save grading scale" : "Create grading scale"}
        </Button>
      </div>
    </form>
  );
}
