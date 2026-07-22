"use client";

import * as React from "react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Input, Label, Select } from "@auraedu/ui";
import { createStaffAction, updateStaffAction, type StaffActionResult } from "@/lib/staff-actions";

type Staff = OpenAPI.staff_v1.components["schemas"]["Staff"];
type User = OpenAPI.identity_v1.components["schemas"]["User"];

interface StaffFormProps {
  mode: "create" | "edit";
  initial?: Staff;
  users: User[];
  onSuccess?: () => void;
}

export function StaffForm({ mode, initial, users, onSuccess }: StaffFormProps) {
  const isEdit = mode === "edit";
  const action = isEdit ? updateStaffAction.bind(null, initial!.id) : createStaffAction;
  const [state, formAction, pending] = React.useActionState<StaffActionResult, FormData>(
    action,
    {},
  );

  React.useEffect(() => {
    if (state.success) onSuccess?.();
  }, [state.success, onSuccess]);

  return (
    <form action={formAction} className="space-y-6">
      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="first_name">First name</Label>
          <Input
            id="first_name"
            name="first_name"
            required
            maxLength={100}
            defaultValue={initial?.first_name}
            placeholder="Ama"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="last_name">Last name</Label>
          <Input
            id="last_name"
            name="last_name"
            required
            maxLength={100}
            defaultValue={initial?.last_name}
            placeholder="Mensah"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="staff_type">Staff type</Label>
          <Select
            id="staff_type"
            name="staff_type"
            required
            defaultValue={initial?.staff_type ?? "teacher"}
          >
            <option value="teacher">Teacher</option>
            <option value="non_teaching">Non-teaching staff</option>
          </Select>
        </div>
        {isEdit ? (
          <div className="space-y-2">
            <Label htmlFor="status">Lifecycle status</Label>
            <Select id="status" name="status" required defaultValue={initial?.status ?? "active"}>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </Select>
          </div>
        ) : null}
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="email">
            Work email{" "}
            <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
          </Label>
          <Input
            id="email"
            name="email"
            type="email"
            maxLength={254}
            defaultValue={initial?.email ?? ""}
            placeholder="ama@school.edu.gh"
          />
        </div>
        <div className="space-y-2 sm:col-span-2">
          <Label htmlFor="user_id">
            Portal account{" "}
            <span className="font-normal text-[var(--muted-foreground)]">(optional)</span>
          </Label>
          <Select id="user_id" name="user_id" defaultValue={initial?.user_id ?? ""}>
            <option value="">No linked portal account</option>
            {users.map((user) => (
              <option key={user.id} value={user.id}>
                {user.name} · {user.email} · {user.role.replaceAll("_", " ")}
              </option>
            ))}
          </Select>
          <p className="text-xs leading-5 text-[var(--muted-foreground)]">
            Linking an active Identity account gives this person their role-scoped web and mobile
            workspace.
          </p>
        </div>
      </div>

      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}
      {state.success ? (
        <p
          role="status"
          className="rounded-xl bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700"
        >
          {isEdit ? "Staff record updated." : "Staff record created."}
        </p>
      ) : null}

      <div className="flex justify-end">
        <Button type="submit" loading={pending} loadingLabel={isEdit ? "Saving" : "Creating"}>
          {isEdit ? "Save staff record" : "Create staff record"}
        </Button>
      </div>
    </form>
  );
}
