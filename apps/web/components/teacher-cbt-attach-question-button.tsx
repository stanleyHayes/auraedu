"use client";

import * as React from "react";
import { ListPlus, ListX } from "lucide-react";
import { Button } from "@auraedu/ui";
import {
  attachQuestionAction,
  detachQuestionAction,
  type TeacherCbtActionResult,
} from "@/lib/teacher-cbt-actions";

interface TeacherCbtAttachQuestionButtonProps {
  examId: string;
  questionId: string;
  attached: boolean;
}

export function TeacherCbtAttachQuestionButton({
  examId,
  questionId,
  attached,
}: TeacherCbtAttachQuestionButtonProps) {
  const action = attached
    ? detachQuestionAction.bind(null, examId, questionId)
    : attachQuestionAction.bind(null, examId, questionId);

  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    action,
    {},
  );

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel={attached ? "Removing" : "Adding"}
        onClick={(e) => {
          if (
            attached &&
            !confirm("Remove this question from the exam? It stays in the question bank.")
          ) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2"
      >
        {attached ? <ListX className="size-4" /> : <ListPlus className="size-4" />}
        <span className="sr-only">{attached ? "Remove from exam" : "Add to exam"}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
