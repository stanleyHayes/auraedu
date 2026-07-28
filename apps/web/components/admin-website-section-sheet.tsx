"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminWebsiteSectionForm } from "./admin-website-section-form";

type WebsiteSection = OpenAPI.website_v1.components["schemas"]["Section"];

interface AdminWebsiteSectionSheetProps {
  mode: "create" | "edit";
  pageId: string;
  initial?: WebsiteSection;
  nextSortOrder: number;
}

export function AdminWebsiteSectionSheet({
  mode,
  pageId,
  initial,
  nextSortOrder,
}: AdminWebsiteSectionSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {initial?.type} section</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Add section
        </Button>
      )}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-2xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">
              {isEdit ? "Edit section" : "Add section"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the content visitors see, or switch the section off without deleting it."
                : "Pick a section type and fill in exactly the content the public site renders."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminWebsiteSectionForm
              mode={mode}
              pageId={pageId}
              sectionId={initial?.id}
              initial={initial}
              nextSortOrder={nextSortOrder}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
