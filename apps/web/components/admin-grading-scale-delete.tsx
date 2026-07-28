"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import {
  deleteGradingScaleAction,
  type AdminGradingActionResult,
} from "@/lib/admin-grading-actions";

export function AdminGradingScaleDelete({ id, name }: { id: string; name: string }) {
  const [state, formAction, pending] = React.useActionState<AdminGradingActionResult, FormData>(
    deleteGradingScaleAction.bind(null, id),
    {},
  );

  return (
    <form
      action={formAction}
      onSubmit={(event) => {
        if (
          !window.confirm(
            `Delete the grading scale "${name}"? Assessments and report cards that reference it will lose their grade bands. This cannot be undone.`,
          )
        ) {
          event.preventDefault();
        }
      }}
    >
      <Button
        type="submit"
        variant="ghost"
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
        loading={pending}
        loadingLabel="Deleting"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete {name}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
