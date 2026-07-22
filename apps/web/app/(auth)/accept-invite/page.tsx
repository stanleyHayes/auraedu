"use client";

import * as React from "react";
import Link from "next/link";
import { CheckCircle2, ShieldCheck, UserRoundPlus } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { acceptInviteAction, type AcceptInviteResult } from "./actions";

export default function AcceptInvitePage() {
  return <AcceptInviteForm />;
}

function AcceptInviteForm() {
  const [token, setToken] = React.useState("");
  const [state, formAction, pending] = React.useActionState<AcceptInviteResult, FormData>(
    acceptInviteAction,
    {},
  );

  React.useEffect(() => {
    const fragment = new URLSearchParams(window.location.hash.slice(1));
    const inviteToken = fragment.get("token") ?? "";
    setToken(inviteToken);
    if (window.location.search || window.location.hash) {
      window.history.replaceState(null, "", "/accept-invite");
    }
  }, []);

  return (
    <div className="relative w-full max-w-[460px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[12rem] opacity-[0.04]">
        Join
      </Watermark>
      <div className="relative rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        {state.success ? (
          <div role="status" className="py-4 text-center">
            <span className="mx-auto grid size-16 place-items-center rounded-2xl bg-[var(--color-positive)]/12 text-[var(--color-positive)]">
              <CheckCircle2 className="size-8" aria-hidden="true" />
            </span>
            <h1 className="mt-5 font-heading text-3xl font-extrabold tracking-tight">
              Your workspace is ready
            </h1>
            <p className="mx-auto mt-3 max-w-sm text-sm leading-6 text-muted-foreground">
              Your administrator account is active and your school has been selected on this device.
            </p>
            <Button asChild className="mt-7 w-full">
              <Link href="/login">Continue to sign in</Link>
            </Button>
          </div>
        ) : (
          <>
            <PageHeader
              icon={<UserRoundPlus className="size-7" />}
              title="Create your administrator account"
              description="Finish the private invitation from AuraEDU, then enter your school workspace."
            />
            <div className="mt-5 flex gap-3 rounded-2xl border border-[var(--color-brand)]/15 bg-[var(--color-brand)]/6 p-3.5">
              <ShieldCheck
                className="mt-0.5 size-5 shrink-0 text-[var(--brand-text)]"
                aria-hidden="true"
              />
              <p className="text-xs leading-5 text-muted-foreground">
                The invitation is single-use and expires automatically. AuraEDU will never ask you
                to send the link or your password by email.
              </p>
            </div>
            <form action={formAction} className="mt-6 space-y-4">
              <input type="hidden" name="token" value={token} />
              <div>
                <label htmlFor="name" className="mb-1.5 block text-sm font-semibold">
                  Full name
                </label>
                <input
                  id="name"
                  name="name"
                  autoComplete="name"
                  minLength={2}
                  maxLength={160}
                  required
                  className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm shadow-sm focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
                />
              </div>
              <div>
                <label htmlFor="password" className="mb-1.5 block text-sm font-semibold">
                  Create password
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  minLength={12}
                  maxLength={256}
                  required
                  aria-describedby="password-help"
                  className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm shadow-sm focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
                />
                <p id="password-help" className="mt-1.5 text-xs text-muted-foreground">
                  Use 12–256 characters and avoid a password used elsewhere.
                </p>
              </div>
              <div>
                <label
                  htmlFor="password_confirmation"
                  className="mb-1.5 block text-sm font-semibold"
                >
                  Confirm password
                </label>
                <input
                  id="password_confirmation"
                  name="password_confirmation"
                  type="password"
                  autoComplete="new-password"
                  minLength={12}
                  maxLength={256}
                  required
                  className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm shadow-sm focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
                />
              </div>
              {!token ? (
                <p
                  role="alert"
                  className="rounded-xl bg-[var(--color-crit)]/10 px-3 py-2 text-sm text-[var(--color-crit)]"
                >
                  Open the complete invitation link from your email.
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
                loadingLabel="Activating account"
                className="w-full"
              >
                Activate account
              </Button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
