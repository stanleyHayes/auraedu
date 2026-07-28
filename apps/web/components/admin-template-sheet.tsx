"use client";

import * as React from "react";
import { MailPlus, Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminTemplateForm } from "./admin-template-form";

type Template = OpenAPI.notification_v1.components["schemas"]["Template"];

export function AdminTemplateSheet({
  mode,
  initial,
}: {
  mode: "create" | "edit";
  initial?: Template;
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
          <Plus className="size-4" /> New template
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
            <MailPlus className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">
              {isEdit ? "Edit template" : "Write a reusable message"}
            </h2>
            <p className="relative mt-1 max-w-lg text-sm leading-6 text-[var(--muted-foreground)]">
              {isEdit
                ? "Changes apply to future sends only; messages already queued keep their rendered content."
                : "Templates start active so journeys and direct sends can use them immediately."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminTemplateForm mode={mode} initial={initial} onSuccess={() => setOpen(false)} />
          </div>
        </div>
      </Sheet>
    </>
  );
}
