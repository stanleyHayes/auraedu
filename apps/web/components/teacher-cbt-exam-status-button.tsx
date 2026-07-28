"use client";

import * as React from "react";
import { CircleStop, Play, Send } from "lucide-react";
import { Button } from "@auraedu/ui";
import { setExamStatusAction, type TeacherCbtActionResult } from "@/lib/teacher-cbt-actions";
import type { Exam } from "@/lib/teacher-cbt-utils";

type ExamStatus = NonNullable<Exam["status"]>;

interface Transition {
  target: ExamStatus;
  verb: string;
  confirm: (title: string) => string;
  icon: React.ReactNode;
}

const TRANSITIONS: Partial<Record<ExamStatus, Transition>> = {
  draft: {
    target: "published",
    verb: "Publish",
    confirm: (title) => `Publish "${title}"? It will be visible to students once activated.`,
    icon: <Send className="size-4" />,
  },
  published: {
    target: "active",
    verb: "Activate",
    confirm: (title) => `Activate "${title}"? Students can begin attempts during the exam window.`,
    icon: <Play className="size-4" />,
  },
  active: {
    target: "closed",
    verb: "Close",
    confirm: (title) => `Close "${title}"? No further attempts will be accepted.`,
    icon: <CircleStop className="size-4" />,
  },
};

interface TeacherCbtExamStatusButtonProps {
  exam: Exam;
}

export function TeacherCbtExamStatusButton({ exam }: TeacherCbtExamStatusButtonProps) {
  const transition = exam.status ? TRANSITIONS[exam.status] : TRANSITIONS.draft;

  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    setExamStatusAction.bind(null, exam.id, transition?.target ?? "draft"),
    {},
  );

  if (!transition) return null;

  return (
    <form action={formAction}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel={transition.verb}
        onClick={(e) => {
          if (!confirm(transition.confirm(exam.title))) {
            e.preventDefault();
          }
        }}
        className="h-8 px-2"
      >
        {transition.icon}
        <span className="sr-only">
          {transition.verb} {exam.title}
        </span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
