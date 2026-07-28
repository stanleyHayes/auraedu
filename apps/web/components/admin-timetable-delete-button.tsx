"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import {
  deleteTimetableEntryAction,
  type AdminTimetableActionResult,
} from "@/lib/admin-timetable-actions";

interface AdminTimetableDeleteButtonProps {
  id: string;
  subjectName: string;
}

export function AdminTimetableDeleteButton({ id, subjectName }: AdminTimetableDeleteButtonProps) {
  const [state, formAction, pending] = React.useActionState<AdminTimetableActionResult, FormData>(
    deleteTimetableEntryAction.bind(null, id),
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
          if (!confirm(`Remove ${subjectName} from the timetable? This cannot be undone.`)) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete {subjectName} period</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
