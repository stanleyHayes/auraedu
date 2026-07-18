"use client";

import * as React from "react";
import { Send } from "lucide-react";
import { Button } from "@auraedu/ui";
import { publishAssignmentAction, type TeacherActionResult } from "@/lib/teacher-actions";

interface PublishAssignmentButtonProps {
  id: string;
  title: string;
}

export function PublishAssignmentButton({ id, title }: PublishAssignmentButtonProps) {
  const [state, formAction, pending] = React.useActionState<TeacherActionResult, FormData>(
    publishAssignmentAction.bind(null, id),
    {},
  );

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel="Publishing"
        onClick={(e) => {
          if (!confirm(`Publish "${title}"? Students will be notified.`)) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2"
      >
        <Send className="size-4" />
        <span className="sr-only">Publish {title}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
