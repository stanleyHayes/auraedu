"use client";

import * as React from "react";
import { Trash2 } from "lucide-react";
import { Button } from "@auraedu/ui";
import { deleteQuestionAction, type TeacherCbtActionResult } from "@/lib/teacher-cbt-actions";

interface TeacherCbtDeleteQuestionButtonProps {
  questionId: string;
  examId: string;
}

export function TeacherCbtDeleteQuestionButton({
  questionId,
  examId,
}: TeacherCbtDeleteQuestionButtonProps) {
  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    deleteQuestionAction.bind(null, questionId, examId),
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
          if (
            !confirm("Delete this question? It is removed from the exam and the question bank.")
          ) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Delete question</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
