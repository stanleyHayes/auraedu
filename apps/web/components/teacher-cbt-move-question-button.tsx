"use client";

import * as React from "react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { Button } from "@auraedu/ui";
import { moveQuestionAction, type TeacherCbtActionResult } from "@/lib/teacher-cbt-actions";

interface TeacherCbtMoveQuestionButtonProps {
  examId: string;
  questionId: string;
  direction: "up" | "down";
  disabled?: boolean;
}

export function TeacherCbtMoveQuestionButton({
  examId,
  questionId,
  direction,
  disabled,
}: TeacherCbtMoveQuestionButtonProps) {
  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    moveQuestionAction.bind(null, examId, questionId, direction),
    {},
  );

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel={direction === "up" ? "Moving up" : "Moving down"}
        disabled={disabled}
        className="h-8 px-2"
      >
        {direction === "up" ? <ArrowUp className="size-4" /> : <ArrowDown className="size-4" />}
        <span className="sr-only">Move question {direction}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
