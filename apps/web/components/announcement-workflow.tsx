"use client";

import * as React from "react";
import { Megaphone, Plus, Trash2 } from "lucide-react";
import { Button, Input, Label, Select, Sheet } from "@auraedu/ui";
import {
  createAnnouncementAction,
  deleteAnnouncementAction,
  type CommunicationActionResult,
} from "@/lib/communication-actions";

export function AnnouncementFormSheet() {
  const [open, setOpen] = React.useState(false);
  const [state, action, pending] = React.useActionState<CommunicationActionResult, FormData>(
    createAnnouncementAction,
    {},
  );
  React.useEffect(() => {
    if (state.success) setOpen(false);
  }, [state.success]);

  return (
    <>
      <Button type="button" onClick={() => setOpen(true)}>
        <Plus className="size-4" />
        Publish announcement
      </Button>
      <Sheet
        open={open}
        onClose={() => setOpen(false)}
        side="right"
        className="w-full max-w-xl bg-[var(--surface)] p-0"
      >
        <div className="flex h-full flex-col">
          <header className="relative overflow-hidden border-b border-[var(--border)] bg-[var(--muted)] px-6 py-5">
            <span
              aria-hidden
              className="absolute -right-12 -top-16 size-40 rounded-full bg-[var(--portal-signal,var(--color-signal))]/14 blur-2xl"
            />
            <div className="relative flex items-start gap-3">
              <span className="grid size-10 place-items-center rounded-xl bg-[var(--portal-accent,var(--color-brand))]/10 text-[var(--portal-accent,var(--color-brand))]">
                <Megaphone className="size-5" />
              </span>
              <div>
                <h2 className="font-heading text-lg font-bold">Publish an announcement</h2>
                <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                  Send one clear update to the right school audience.
                </p>
              </div>
            </div>
          </header>
          <form action={action} className="flex-1 space-y-5 overflow-y-auto p-6">
            <div className="space-y-2">
              <Label htmlFor="title">Announcement title</Label>
              <Input
                id="title"
                name="title"
                required
                minLength={3}
                maxLength={160}
                placeholder="Term 2 reopening arrangements"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="audience">Audience</Label>
              <Select id="audience" name="audience" defaultValue="all">
                <option value="all">Everyone</option>
                <option value="students">Students</option>
                <option value="guardians">Parents and guardians</option>
                <option value="staff">Staff</option>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="body">Message</Label>
              <textarea
                id="body"
                name="body"
                required
                minLength={3}
                maxLength={5000}
                rows={9}
                placeholder="Share the verified information, important dates, and next action…"
                className="w-full resize-y rounded-xl border border-[var(--border)] bg-[var(--background)] px-3.5 py-3 text-sm leading-6 outline-none transition focus:border-[var(--portal-accent,var(--color-brand))] focus:ring-2 focus:ring-[var(--ring)]/35"
              />
              <p className="text-xs leading-5 text-[var(--muted-foreground)]">
                This appears in the role-scoped web and mobile announcement inbox. Confirm names,
                dates, and audience before publishing.
              </p>
            </div>
            {state.error ? (
              <p role="alert" className="rounded-xl bg-red-500/10 px-4 py-3 text-sm text-red-700">
                {state.error}
              </p>
            ) : null}
            <div className="flex justify-end">
              <Button type="submit" loading={pending} loadingLabel="Publishing">
                Publish announcement
              </Button>
            </div>
          </form>
        </div>
      </Sheet>
    </>
  );
}

export function DeleteAnnouncementButton({ id, title }: { id: string; title: string }) {
  const [state, action, pending] = React.useActionState<CommunicationActionResult, FormData>(
    deleteAnnouncementAction.bind(null, id),
    {},
  );
  return (
    <form action={action}>
      <Button
        type="submit"
        variant="ghost"
        loading={pending}
        loadingLabel="Removing"
        className="h-8 px-2 text-destructive hover:bg-destructive/10"
        onClick={(event) => {
          if (!confirm(`Remove “${title}”? This removes it from the announcement record.`))
            event.preventDefault();
        }}
      >
        <Trash2 className="size-4" />
        <span className="sr-only">Remove {title}</span>
      </Button>
      {state.error ? <span className="sr-only">{state.error}</span> : null}
    </form>
  );
}
