"use client";

import * as React from "react";
import { Pencil, Plus } from "lucide-react";
import { Button, Sheet } from "@auraedu/ui";
import { TeacherCbtQuestionForm } from "./teacher-cbt-question-form";
import type { Exam, Question } from "@/lib/teacher-cbt-utils";

interface TeacherCbtQuestionFormSheetProps {
  mode: "create" | "edit";
  exam: Exam;
  initial?: Question;
}

export function TeacherCbtQuestionFormSheet({
  mode,
  exam,
  initial,
}: TeacherCbtQuestionFormSheetProps) {
  const [open, setOpen] = React.useState(false);
  const isEdit = mode === "edit";

  return (
    <>
      {isEdit ? (
        <Button type="button" variant="ghost" className="h-8 px-2" onClick={() => setOpen(true)}>
          <Pencil className="size-4" />
          <span className="sr-only">Edit question</span>
        </Button>
      ) : (
        <Button type="button" onClick={() => setOpen(true)}>
          <Plus className="mr-2 size-4" />
          New question
        </Button>
      )}
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="border-b border-[var(--border)] bg-[var(--muted)] px-6 py-4">
            <h2 className="font-heading text-lg font-bold">
              {isEdit ? "Edit question" : "New question"}
            </h2>
            <p className="text-sm text-muted-foreground">
              {isEdit
                ? "Update the wording, options, correct answer, or marks."
                : `Author a question and add it to "${exam.title}".`}
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <TeacherCbtQuestionForm
              mode={mode}
              exam={exam}
              questionId={initial?.id}
              initial={initial}
              onSuccess={() => setOpen(false)}
            />
          </div>
        </div>
      </Sheet>
    </>
  );
}
