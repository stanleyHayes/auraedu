"use client";

import * as React from "react";
import { ShieldCheck, UserRoundX } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Select } from "@auraedu/ui";
import {
  assignRoleAction,
  deactivateUserAction,
  type AdminUserActionResult,
} from "@/lib/admin-user-actions";

type User = OpenAPI.identity_v1.components["schemas"]["User"];

export function AdminUserRoleActions({ user, roles }: { user: User; roles: string[] }) {
  const assign = React.useMemo(() => assignRoleAction.bind(null, user.id), [user.id]);
  const deactivate = React.useMemo(() => deactivateUserAction.bind(null, user.id), [user.id]);
  const [assignState, assignAction, assignPending] = React.useActionState<
    AdminUserActionResult,
    FormData
  >(assign, {});
  const [deactivateState, deactivateAction, deactivatePending] = React.useActionState<
    AdminUserActionResult,
    FormData
  >(deactivate, {});

  const roleChoices = roles.length > 0 ? roles : [user.role];
  const error = assignState.error ?? deactivateState.error;

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <form action={assignAction} className="flex items-center gap-2">
          <Select
            aria-label={`Role for ${user.name}`}
            name="role"
            defaultValue={user.role}
            disabled={roles.length === 0}
            className="h-8 w-40 text-xs"
          >
            {roleChoices.map((role) => (
              <option key={role} value={role}>
                {role.replaceAll("_", " ")}
              </option>
            ))}
          </Select>
          <Button
            type="submit"
            variant="secondary"
            className="h-8 gap-1.5 px-2.5 text-xs"
            loading={assignPending}
            loadingLabel="Saving"
            disabled={roles.length === 0}
          >
            <ShieldCheck className="size-3.5" /> Reassign
          </Button>
        </form>
        <form
          action={deactivateAction}
          onSubmit={(event) => {
            if (
              !window.confirm(
                `Deactivate ${user.name} (${user.email})? They will lose access immediately. This cannot be undone from this console.`,
              )
            ) {
              event.preventDefault();
            }
          }}
        >
          <Button
            type="submit"
            variant="ghost"
            className="h-8 gap-1.5 px-2.5 text-xs text-red-700 hover:bg-red-500/10"
            loading={deactivatePending}
            loadingLabel="Deactivating"
          >
            <UserRoundX className="size-3.5" /> Deactivate
          </Button>
        </form>
      </div>
      {error ? (
        <p role="alert" className="max-w-md text-right text-xs leading-5 text-red-700">
          {error}
        </p>
      ) : null}
    </div>
  );
}
