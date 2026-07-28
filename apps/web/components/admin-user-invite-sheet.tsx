"use client";

import * as React from "react";
import { UserRoundPlus } from "lucide-react";
import { Button, Input, Label, Select, Sheet } from "@auraedu/ui";
import { inviteUserAction, type AdminUserActionResult } from "@/lib/admin-user-actions";

/** Roles whose sign-in the identity service gates behind a TOTP challenge. */
const MFA_ENFORCED_ROLES = new Set([
  "platform_super_admin",
  "school_admin",
  "principal",
  "academic_head",
  "accountant",
  "support_agent",
]);

export function AdminUserInviteSheet({ roles }: { roles: string[] }) {
  const [open, setOpen] = React.useState(false);
  const [selectedRole, setSelectedRole] = React.useState(roles[0] ?? "");
  const [state, formAction, pending] = React.useActionState<AdminUserActionResult, FormData>(
    inviteUserAction,
    {},
  );
  React.useEffect(() => {
    if (state.success) setOpen(false);
  }, [state.success]);

  return (
    <>
      <Button type="button" onClick={() => setOpen(true)} disabled={roles.length === 0}>
        <UserRoundPlus className="size-4" /> Invite user
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_88%,var(--portal-accent-soft))] px-6 py-6">
            <span className="absolute -right-10 -top-14 size-36 rounded-full bg-[var(--portal-accent)]/10 blur-2xl" />
            <UserRoundPlus className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">Invite a user</h2>
            <p className="relative mt-1 max-w-lg text-sm leading-6 text-[var(--muted-foreground)]">
              The invitation is delivered privately and expires after first use. The invitee sets
              their own name and password when accepting it.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <form action={formAction} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="invite_email">Email address</Label>
                <Input
                  id="invite_email"
                  name="email"
                  type="email"
                  required
                  maxLength={254}
                  placeholder="teacher@school.edu.gh"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="invite_role">Role</Label>
                <Select
                  id="invite_role"
                  name="role"
                  value={selectedRole}
                  onChange={(event) => setSelectedRole(event.target.value)}
                >
                  {roles.map((role) => (
                    <option key={role} value={role}>
                      {role.replaceAll("_", " ")}
                    </option>
                  ))}
                </Select>
                <p className="text-xs leading-5 text-[var(--muted-foreground)]">
                  You can only grant roles and permissions you hold yourself; the identity service
                  rejects anything beyond that.
                </p>
              </div>
              {MFA_ENFORCED_ROLES.has(selectedRole) ? (
                <p className="rounded-xl border border-amber-300/60 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-950">
                  {selectedRole.replaceAll("_", " ")} is a privileged role: the identity service
                  requires the invitee to enrol an authenticator app (TOTP) before their first
                  sign-in completes.
                </p>
              ) : null}
              {state.error ? (
                <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
                  {state.error}
                </p>
              ) : null}
              <div className="flex justify-end">
                <Button type="submit" loading={pending} loadingLabel="Sending">
                  Send invitation
                </Button>
              </div>
            </form>
          </div>
        </div>
      </Sheet>
    </>
  );
}
