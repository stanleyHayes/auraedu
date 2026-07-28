"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import { deleteExamAction, type TeacherCbtActionResult } from "@/lib/teacher-cbt-actions";

interface TeacherCbtDeleteExamButtonProps {
  id: string;
  title: string;
}

export function TeacherCbtDeleteExamButton({ id, title }: TeacherCbtDeleteExamButtonProps) {
  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    deleteExamAction.bind(null, id),
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
