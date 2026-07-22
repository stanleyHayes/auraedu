"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import {
  createStudentAction,
  updateStudentAction,
  type StudentActionResult,
} from "@/lib/student-actions";

type Student = OpenAPI.student_v1.components["schemas"]["Student"];
type AcademicClass = OpenAPI.academic_v1.components["schemas"]["Class"];
type AcademicYear = OpenAPI.academic_v1.components["schemas"]["AcademicYear"];
type User = OpenAPI.identity_v1.components["schemas"]["User"];

export function StudentForm({
  mode,
  initial,
  classes,
  years,
  users,
  onSuccess,
}: {
  mode: "create" | "edit";
  initial?: Student;
  classes: AcademicClass[];
  years: AcademicYear[];
  users: User[];
  onSuccess?: () => void;
}) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateStudentAction.bind(null, initial!.id) : createStudentAction;
  const [state, formAction, pending] = React.useActionState<StudentActionResult, FormData>(
    action,
    {},
  );
  React.useEffect(() => {
    if (state.success) onSuccess?.();
  }, [state.success, onSuccess]);

  return (
    <form action={formAction} className="space-y-6">
      <div className="grid gap-5 sm:grid-cols-2">
        <Field
          label="First name"
          name="first_name"
          defaultValue={initial?.first_name}
          placeholder="Kofi"
        />
        <Field
          label="Last name"
          name="last_name"
          defaultValue={initial?.last_name}
          placeholder="Owusu"
        />
        {isEdit ? (
          <div className="space-y-2 sm:col-span-2">
            <Label htmlFor="status">Learner status</Label>
            <Select id="status" name="status" defaultValue={initial?.status ?? "active"}>
              <option value="active">Active</option>
              <option value="graduated">Graduated</option>
              <option value="withdrawn">Withdrawn</option>
            </Select>
          </div>
        ) : (
          <>
            <div className="space-y-2">
              <Label htmlFor="date_of_birth">
                Date of birth{" "}
                <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
              </Label>
              <Input
                id="date_of_birth"
                name="date_of_birth"
                type="date"
                max={new Date().toISOString().slice(0, 10)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="gender">
                Gender{" "}
                <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
              </Label>
              <Select id="gender" name="gender" defaultValue="">
                <option value="">Not recorded</option>
                <option value="female">Female</option>
                <option value="male">Male</option>
                <option value="other">Other</option>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="academic_year_id">Initial academic year</Label>
              <Select id="academic_year_id" name="academic_year_id" defaultValue="">
                <option value="">Enrol later</option>
                {years.map((year) => (
                  <option key={year.id} value={year.id}>
                    {year.name}
                    {year.is_current ? " · current" : ""}
                  </option>
                ))}
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="class_id">Initial class</Label>
              <Select id="class_id" name="class_id" defaultValue="">
                <option value="">Enrol later</option>
                {classes.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </Select>
            </div>
          </>
        )}
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="user_id">
            Student portal account{" "}
            <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
          </Label>
          <Select id="user_id" name="user_id" defaultValue={initial?.user_id ?? ""}>
            <option value="">No linked account</option>
            {users.map((user) => (
              <option key={user.id} value={user.id}>
                {user.name} · {user.email}
              </option>
            ))}
          </Select>
          <p className="text-xs leading-5 text-[var(--muted-foreground)]">
            Only active tenant Identity accounts are offered. The API remains the authority for
            learner ownership.
          </p>
        </div>
      </div>
      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}
      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Enrolling"}>
          {isEdit ? "Save learner record" : "Create learner"}
        </Button>
      </div>
    </form>
  );
}

function Field({
  label,
  name,
  defaultValue,
  placeholder,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  placeholder: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={name}>{label}</Label>
      <Input
        id={name}
        name={name}
        required
        maxLength={100}
        defaultValue={defaultValue}
        placeholder={placeholder}
      />
    </div>
  );
}
