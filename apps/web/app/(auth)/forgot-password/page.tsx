"use client";

import * as React from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, MailCheck } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { WorkspaceField } from "@/components/workspace-field";
import { forgotPasswordAction, type ForgotPasswordResult } from "./actions";

export default function ForgotPasswordPage() {
  return (
    <React.Suspense fallback={null}>
      <ForgotPasswordForm />
    </React.Suspense>
  );
}

function ForgotPasswordForm() {
  const searchParams = useSearchParams();
  const [state, formAction, pending] = React.useActionState<ForgotPasswordResult, FormData>(
    forgotPasswordAction,
    {},
  );
  return (
    <div className="relative w-full max-w-[440px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[11rem] opacity-[0.04]">
        Reset
      </Watermark>
      <div className="relative rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        {state.success ? (
          <div role="status" className="py-4 text-center">
            <span className="mx-auto grid size-16 place-items-center rounded-2xl bg-[var(--color-positive)]/12 text-[var(--color-positive)]">
              <CheckCircle2 className="size-8" aria-hidden="true" />
            </span>
            <h1 className="mt-5 font-heading text-3xl font-extrabold tracking-tight">
              Check your inbox
            </h1>
            <p className="mx-auto mt-3 max-w-sm text-sm leading-6 text-muted-foreground">
              If that account exists in this school, AuraEDU has sent a private, time-limited
              recovery link.
            </p>
            <Button asChild variant="secondary" className="mt-7 w-full">
              <Link href="/login">Return to sign in</Link>
            </Button>
          </div>
        ) : (
          <>
            <PageHeader
              icon={<MailCheck className="size-7" />}
              title="Recover your account"
              description="Enter the school workspace and email used for your AuraEDU account."
            />
            <form action={formAction} className="mt-6 space-y-4">
              <WorkspaceField defaultValue={searchParams.get("tenant") ?? ""} />
              <div>
                <label htmlFor="email" className="mb-1.5 block text-sm font-semibold">
                  Email
                </label>
                <input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm shadow-sm focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
                />
              </div>
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
                loading={pending}
                loadingLabel="Sending recovery link"
                className="w-full"
              >
                Send recovery link
              </Button>
              <p className="text-center text-sm">
                <Link
                  href="/login"
                  className="font-semibold text-[var(--brand-text)] hover:underline"
                >
                  Back to sign in
                </Link>
              </p>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
