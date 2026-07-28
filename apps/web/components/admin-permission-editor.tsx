"use client";

import * as React from "react";
import { KeyRound, ShieldAlert } from "lucide-react";
import type { OpenAPI } from "@auraedu/shared-types";
import { Button, Sheet } from "@auraedu/ui";
import {
  updateUserPermissionsAction,
  type AdminPermissionActionResult,
} from "@/lib/admin-permission-actions";
import { groupPermissionsByResource, permissionActionLabel } from "@/lib/admin-permission-groups";

type User = OpenAPI.identity_v1.components["schemas"]["User"];

export function AdminPermissionEditor({ user, catalogue }: { user: User; catalogue: string[] }) {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        className="h-8 gap-1.5 px-2.5 text-xs"
        disabled={catalogue.length === 0}
        onClick={() => setOpen(true)}
      >
        <KeyRound className="size-3.5" /> Permissions
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-2xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <div className="relative overflow-hidden border-b border-[var(--border)] bg-[color-mix(in_oklab,var(--surface)_88%,var(--portal-accent-soft))] px-6 py-6">
            <span className="absolute -right-10 -top-14 size-36 rounded-full bg-[var(--portal-accent)]/10 blur-2xl" />
            <KeyRound className="relative size-6 text-[var(--portal-accent)]" />
            <h2 className="relative mt-3 text-xl font-black tracking-tight">
              Permissions for {user.name}
            </h2>
            <p className="relative mt-1 max-w-lg text-sm leading-6 text-[var(--muted-foreground)]">
              Role <span className="font-semibold">{user.role.replaceAll("_", " ")}</span> · the
              ticked keys are the explicit grants this account currently holds.
            </p>
          </div>
          <div className="flex-1 overflow-y-auto p-6">
            <PermissionMatrix user={user} catalogue={catalogue} onDone={() => setOpen(false)} />
          </div>
        </div>
      </Sheet>
    </>
  );
}

function PermissionMatrix({
  user,
  catalogue,
  onDone,
}: {
  user: User;
  catalogue: string[];
  onDone: () => void;
}) {
  const action = React.useMemo(() => updateUserPermissionsAction.bind(null, user.id), [user.id]);
  const [state, formAction, pending] = React.useActionState<AdminPermissionActionResult, FormData>(
    action,
    {},
  );
  const [selected, setSelected] = React.useState<Set<string>>(
    () => new Set(user.permissions ?? []),
  );
  React.useEffect(() => {
    if (state.success) onDone();
  }, [state.success, onDone]);

  const groups = React.useMemo(() => groupPermissionsByResource(catalogue), [catalogue]);
  const current = React.useMemo(() => new Set(user.permissions ?? []), [user.permissions]);
  const dirty = React.useMemo(() => {
    if (selected.size !== current.size) return true;
    for (const key of selected) if (!current.has(key)) return true;
    return false;
  }, [selected, current]);

  function toggle(key: string) {
    setSelected((previous) => {
      const next = new Set(previous);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }

  return (
    <form action={formAction} className="space-y-6">
      <div className="flex items-start gap-3 rounded-xl border border-amber-300/60 bg-amber-50 p-4 text-sm leading-6 text-amber-950">
        <ShieldAlert className="mt-0.5 size-4 shrink-0" />
        <p>
          Grants are validated server-side: you can only grant permissions you hold yourself, and
          saving replaces this account&apos;s whole grant set. Unchecking every key leaves the
          account with no explicit permissions.
        </p>
      </div>

      <div className="space-y-5">
        {groups.map((group) => (
          <fieldset key={group.resource} className="space-y-2">
            <legend className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
              {group.label}
            </legend>
            <div className="grid gap-1.5 sm:grid-cols-2">
              {group.keys.map((key) => {
                const checked = selected.has(key);
                return (
                  <label
                    key={key}
                    className={`flex cursor-pointer items-center gap-2.5 rounded-lg border px-3 py-2 text-sm transition ${
                      checked
                        ? "border-primary/40 bg-primary/5 font-semibold"
                        : "border-border bg-background/60"
                    }`}
                  >
                    <input
                      type="checkbox"
                      name="permission"
                      value={key}
                      checked={checked}
                      onChange={() => toggle(key)}
                      className="size-4 accent-[var(--primary)]"
                    />
                    <span>{permissionActionLabel(key)}</span>
                    <code className="ml-auto font-mono text-[10px] text-muted-foreground">
                      {key}
                    </code>
                  </label>
                );
              })}
            </div>
          </fieldset>
        ))}
      </div>

      {state.error ? (
        <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
          {state.error}
        </p>
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">
          {selected.size} of {catalogue.length} keys granted
        </p>
        <div className="flex gap-2">
          <Button type="button" variant="ghost" onClick={onDone}>
            Cancel
          </Button>
          <Button type="submit" loading={pending} loadingLabel="Saving" disabled={!dirty}>
            Save grants
          </Button>
        </div>
      </div>
    </form>
  );
}
