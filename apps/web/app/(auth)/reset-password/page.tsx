"use client";

import * as React from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, KeyRound } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { WorkspaceField } from "@/components/workspace-field";
import { resetPasswordAction, type ResetPasswordResult } from "./actions";

export default function ResetPasswordPage() {
  return (
    <React.Suspense fallback={null}>
      <ResetPasswordForm />
    </React.Suspense>
  );
}

function ResetPasswordForm() {
  const searchParams = useSearchParams();
  const [token, setToken] = React.useState("");
  const [state, formAction, pending] = React.useActionState<ResetPasswordResult, FormData>(
    resetPasswordAction,
    {},
  );
  React.useEffect(() => {
    setToken(new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "");
    if (window.location.search || window.location.hash)
      window.history.replaceState(null, "", "/reset-password");
  }, []);
  return (
    <div className="relative w-full max-w-[440px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[11rem] opacity-[0.04]">
        Secure
      </Watermark>
      <div className="relative rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        {state.success ? (
          <div role="status" className="py-4 text-center">
            <span className="mx-auto grid size-16 place-items-center rounded-2xl bg-[var(--color-positive)]/12 text-[var(--color-positive)]">
              <CheckCircle2 className="size-8" aria-hidden="true" />
            </span>
            <h1 className="mt-5 font-heading text-3xl font-extrabold tracking-tight">
              Password updated
            </h1>
            <p className="mx-auto mt-3 max-w-sm text-sm leading-6 text-muted-foreground">
              Existing sessions have been closed. Sign in again with your new password.
            </p>
            <Button asChild className="mt-7 w-full">
              <Link href="/login">Continue to sign in</Link>
            </Button>
          </div>
        ) : (
          <>
            <PageHeader
              icon={<KeyRound className="size-7" />}
              title="Choose a new password"
              description="Complete this time-limited recovery inside your school workspace."
            />
            <form action={formAction} className="mt-6 space-y-4">
              <input type="hidden" name="token" value={token} />
              <WorkspaceField defaultValue={searchParams.get("tenant") ?? ""} />
              <div>
                <label htmlFor="password" className="mb-1.5 block text-sm font-semibold">
                  New password
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  minLength={12}
                  maxLength={256}
                  required
                  aria-describedby="reset-password-help"
                  className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm shadow-sm focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
                />
                <p id="reset-password-help" className="mt-1.5 text-xs text-muted-foreground">
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
                  Open the complete recovery link from your email.
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
                loadingLabel="Updating password"
                className="w-full"
              >
                Update password
              </Button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
