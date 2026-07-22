"use client";

import * as React from "react";
import { Plus } from "lucide-react";
import { Button, Sheet } from "@auraedu/ui";
import { TenantForm } from "./tenant-form";

export function TenantCreateSheet() {
  const [open, setOpen] = React.useState(false);

  return (
    <>
      <Button type="button" onClick={() => setOpen(true)}>
        <Plus className="mr-2 size-4" />
        Add school
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">Add school</h2>
            <p className="text-sm text-muted-foreground">
              Create a new school tenant. The tenant code becomes the subdomain.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <TenantForm mode="create" onSuccess={() => setOpen(false)} />
          </div>
        </div>
      </Sheet>
    </>
  );
}
