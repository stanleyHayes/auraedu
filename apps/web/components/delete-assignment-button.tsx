"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import { deleteAssignmentAction, type TeacherActionResult } from "@/lib/teacher-actions";

interface DeleteAssignmentButtonProps {
  id: string;
  title: string;
}

export function DeleteAssignmentButton({ id, title }: DeleteAssignmentButtonProps) {
  const [state, formAction, pending] = React.useActionState<TeacherActionResult, FormData>(
    deleteAssignmentAction.bind(null, id),
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
          if (!confirm(`Delete "${title}"? This cannot be undone.`)) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete {title}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
