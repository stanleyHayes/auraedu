"use client";

import * as React from "react";
import Link from "next/link";
import { BellOff, CheckCircle2, ShieldCheck } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { unsubscribeAction, type UnsubscribeResult } from "./actions";

export default function UnsubscribePage() {
  const [token, setToken] = React.useState("");
  const [state, formAction, pending] = React.useActionState<UnsubscribeResult, FormData>(
    unsubscribeAction,
    {},
  );

  React.useEffect(() => {
    setToken(new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "");
    if (window.location.search || window.location.hash)
      window.history.replaceState(null, "", "/unsubscribe");
  }, []);

  return (
    <div className="relative w-full max-w-[460px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[10rem] opacity-[0.04]">
        Choice
      </Watermark>
      <div className="relative overflow-hidden rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        <span
          aria-hidden="true"
          className="absolute -right-20 -top-20 size-44 rounded-full bg-[var(--color-teal-bright)]/10 blur-3xl motion-safe:animate-pulse"
        />
        {state.success ? (
          <div role="status" className="relative py-4 text-center">
            <span className="mx-auto grid size-16 place-items-center rounded-2xl bg-[var(--color-positive)]/12 text-[var(--color-positive)]">
              <CheckCircle2 className="size-8" aria-hidden="true" />
            </span>
            <h1 className="mt-5 font-heading text-3xl font-extrabold tracking-tight">
              Preference saved
            </h1>
            <p className="mx-auto mt-3 max-w-sm text-sm leading-6 text-muted-foreground">
              Admissions and nurture emails from this school have stopped. Essential account and
              security messages can still reach you.
            </p>
            <Button asChild variant="secondary" className="mt-7 w-full">
              <Link href="/login">Return to AuraEDU</Link>
            </Button>
          </div>
        ) : (
          <div className="relative">
            <PageHeader
              icon={<BellOff className="size-7" />}
              title="Stop admissions updates"
              description="You are in control of the messages you receive from a school using AuraEDU."
            />
            <div className="mt-5 flex gap-3 rounded-2xl border border-[var(--color-brand)]/15 bg-[var(--color-brand)]/6 p-3.5">
              <ShieldCheck
                className="mt-0.5 size-5 shrink-0 text-[var(--brand-text)]"
                aria-hidden="true"
              />
              <p className="text-xs leading-5 text-muted-foreground">
                This private link contains no email address. Confirming creates a one-way
                suppression so future admissions journeys cannot contact that address.
              </p>
            </div>
            <form action={formAction} className="mt-6 space-y-4">
              <input type="hidden" name="token" value={token} />
              {!token ? (
                <p
                  role="alert"
                  className="rounded-xl bg-[var(--color-crit)]/10 px-3 py-2 text-sm text-[var(--color-crit)]"
                >
                  Open the complete preference link from your email.
                </p>
              ) : null}
              {state.error ? (
                <p
                  role="alert"
                  className="rounded-xl bg-[var(--color-crit)]/10 px-3 py-2 text-sm text-[var(--color-crit)]"
                >
                  {state.error}
                </p>
              ) : null}
              <Button
                type="submit"
                disabled={!token}
                loading={pending}
                loadingLabel="Saving preference"
                className="w-full"
              >
                Stop admissions emails
              </Button>
              <p className="text-center text-xs leading-5 text-muted-foreground">
                This does not close an application or disable security notifications.
              </p>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
