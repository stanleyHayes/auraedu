"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import { deleteTenantAction, type TenantActionResult } from "@/lib/tenant-actions";

interface DeleteTenantButtonProps {
  code: string;
  name: string;
}

export function DeleteTenantButton({ code, name }: DeleteTenantButtonProps) {
  const [state, formAction, pending] = React.useActionState<TenantActionResult, FormData>(
    deleteTenantAction.bind(null, code),
    {},
  );

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel="Deleting"
        onClick={(e) => {
          if (!confirm(`Delete "${name}"? This cannot be undone.`)) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete {name}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
