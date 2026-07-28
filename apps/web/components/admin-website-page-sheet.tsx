"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import { AdminWebsitePageForm } from "./admin-website-page-form";

type WebsitePage = OpenAPI.website_v1.components["schemas"]["Page"];

interface AdminWebsitePageSheetProps {
  mode: "create" | "edit";
  initial?: WebsitePage;
}

export function AdminWebsitePageSheet({ mode, initial }: AdminWebsitePageSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit {initial?.title}</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          Add page
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
            <h2 className="font-heading text-lg font-bold">{isEdit ? "Edit page" : "Add page"}</h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Rename the page, change its layout, or take it offline."
                : "Create a page on the public school website, then add sections to it."}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <AdminWebsitePageForm
              mode={mode}
              pageId={initial?.id}
              initial={initial}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
