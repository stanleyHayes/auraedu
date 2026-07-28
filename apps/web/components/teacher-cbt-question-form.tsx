"use client";

import * as React from "react";
import { Plus, X } from "lucide-react";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createQuestionAction,
  updateQuestionAction,
  type TeacherCbtActionResult,
} from "@/lib/teacher-cbt-actions";
import {
  QUESTION_TYPES,
  questionTypeLabel,
  type Exam,
  type Question,
  type QuestionType,
} from "@/lib/teacher-cbt-utils";

interface TeacherCbtQuestionFormProps {
  mode: "create" | "edit";
  exam: Exam;
  questionId?: string;
  initial?: Question;
  onSuccess?: () => void;
}

export function TeacherCbtQuestionForm({
  mode,
  exam,
  questionId,
  initial,
  onSuccess,
}: TeacherCbtQuestionFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit
    ? updateQuestionAction.bind(null, questionId!, exam.id)
    : createQuestionAction.bind(null, exam.id);

  const [state, formAction, pending] = React.useActionState<TeacherCbtActionResult, FormData>(
    action,
    {},
  );
  const [questionType, setQuestionType] = React.useState<QuestionType>(
    initial?.question_type ?? "multiple_choice",
  );
  const [options, setOptions] = React.useState<string[]>(
    initial?.options && initial.options.length > 0 ? initial.options : ["", ""],
  );
  const [correctAnswer, setCorrectAnswer] = React.useState(initial?.correct_answer ?? "");

  React.useEffect(() => {
    if (state.success && onSuccess) {
      onSuccess();
    }
  }, [state, onSuccess]);

  const filledOptions = options
    .map((option) => option.trim())
    .filter((option) => option.length > 0);

  function updateOption(index: number, value: string) {
    setOptions((current) => current.map((option, i) => (i === index ? value : option)));
  }

  function removeOption(index: number) {
    setOptions((current) => current.filter((_, i) => i !== index));
  }

  return (
    <form action={formAction} className="space-y-5">
      <input type="hidden" name="subject_id" value={exam.subject_id} />
      <input type="hidden" name="academic_year_id" value={exam.academic_year_id ?? ""} />

      <div className="space-y-1.5">
        <Label htmlFor="question_text">Question</Label>
        <Input
          id="question_text"
          name="question_text"
          defaultValue={initial?.question_text}
          required
          placeholder="What is 3/4 of 24?"
        />
      </div>

      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="question_type">Type</Label>
          <Select
            id="question_type"
            name="question_type"
            value={questionType}
            onChange={(event) => setQuestionType(event.target.value as QuestionType)}
            required
          >
            {QUESTION_TYPES.map((type) => (
              <option key={type} value={type}>
                {questionTypeLabel(type)}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="marks">Marks</Label>
          <Input
            id="marks"
            name="marks"
            type="number"
            min={1}
            step={1}
            defaultValue={initial?.marks ?? ""}
            required
            placeholder="1"
          />
        </div>
      </div>

      {questionType === "multiple_choice" ? (
        <fieldset className="space-y-3">
          <legend className="text-sm font-medium">Options</legend>
          {options.map((option, index) => (
            <div key={index} className="flex items-center gap-2">
              <Input
                name="option"
                value={option}
                onChange={(event) => updateOption(index, event.target.value)}
                placeholder={`Option ${index + 1}`}
                aria-label={`Option ${index + 1}`}
              />
              <Button
                type="button"
                variant="ghost"
                className="h-8 px-2"
                disabled={options.length <= 2}
                onClick={() => removeOption(index)}
              >
                <X className="size-4" />
                <span className="sr-only">Remove option {index + 1}</span>
              </Button>
            </div>
          ))}
          <Button
            type="button"
            variant="ghost"
            className="h-8 px-2"
            onClick={() => setOptions((current) => [...current, ""])}
          >
            <Plus className="size-4" />
            Add option
          </Button>
        </fieldset>
      ) : null}

      <div className="space-y-1.5">
        <Label htmlFor="correct_answer">Correct answer</Label>
        {questionType === "multiple_choice" ? (
          <Select
            id="correct_answer"
            name="correct_answer"
            value={correctAnswer}
            onChange={(event) => setCorrectAnswer(event.target.value)}
            required
          >
            <option value="" disabled>
              Select the correct option
            </option>
            {filledOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        ) : questionType === "true_false" ? (
          <Select
            id="correct_answer"
            name="correct_answer"
            value={correctAnswer}
            onChange={(event) => setCorrectAnswer(event.target.value)}
            required
          >
            <option value="" disabled>
              Select the correct answer
            </option>
            <option value="True">True</option>
            <option value="False">False</option>
          </Select>
        ) : (
          <Input
            id="correct_answer"
            name="correct_answer"
            value={correctAnswer}
            onChange={(event) => setCorrectAnswer(event.target.value)}
            required
            placeholder="18"
          />
        )}
        {questionType === "short_answer" ? (
          <p className="text-xs text-muted-foreground">
            Auto-grading matches the student&apos;s answer against this text.
          </p>
        ) : null}
      </div>

      {state.error ? (
        <p className="rounded-[var(--radius-sm)] bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p className="rounded-[var(--radius-sm)] bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600">
          {isEdit ? "Question saved." : "Question added."}
        </p>
      ) : null}

      <div className="flex justify-end gap-3 pt-2">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Adding"}>
          {isEdit ? "Save changes" : "Add question"}
        </Button>
      </div>
    </form>
  );
}
