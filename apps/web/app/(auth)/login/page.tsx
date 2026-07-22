"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { KeyRound, ShieldCheck } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { loginAction, verifyMFAAction, type LoginResult } from "./actions";
import { WorkspaceField } from "@/components/workspace-field";

export default function LoginPage() {
  return (
    <React.Suspense fallback={null}>
      <LoginForm />
    </React.Suspense>
  );
}

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [state, formAction, pending] = React.useActionState<LoginResult, FormData>(loginAction, {});

  React.useEffect(() => {
    if (state.success && state.redirectTo) {
      router.push(state.redirectTo);
    }
  }, [state, router]);

  if (state.mfa) {
    return <MFAForm challenge={state.mfa} />;
  }

  return (
    <div className="relative w-full max-w-[420px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[12rem] opacity-[0.04]">
        Aura
      </Watermark>
      <div className="relative rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        <PageHeader
          icon={<KeyRound className="size-7" />}
          title="Welcome back"
          description="Sign in with the email and password from your school administrator."
        />
        <form action={formAction} className="mt-7 space-y-4">
          <input type="hidden" name="next" value={searchParams.get("next") ?? ""} />
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
              placeholder="you@school.edu"
              required
              className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
            />
          </div>
          <div>
            <label htmlFor="password" className="mb-1.5 block text-sm font-semibold">
              Password
            </label>
            <input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              required
              className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-sm text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
            />
          </div>
          {state.error ? (
            <p className="rounded-[var(--radius-sm)] bg-[var(--color-crit)]/10 px-3 py-2 text-sm text-[var(--color-crit)]">
              {state.error}
            </p>
          ) : null}
          <Button type="submit" loading={pending} loadingLabel="Signing in" className="w-full">
            Sign in
          </Button>
          <p className="text-center text-sm">
            <Link
              href={
                searchParams.get("tenant")
                  ? `/forgot-password?tenant=${encodeURIComponent(searchParams.get("tenant")!)}`
                  : "/forgot-password"
              }
              className="font-semibold text-[var(--brand-text)] hover:underline"
            >
              Forgot your password?
            </Link>
          </p>
          <p className="text-center text-xs text-muted-foreground">
            No public sign-up — contact your school administrator for an account.
          </p>
        </form>
      </div>
    </div>
  );
}

function MFAForm({ challenge }: { challenge: NonNullable<LoginResult["mfa"]> }) {
  const router = useRouter();
  const [state, formAction, pending] = React.useActionState<LoginResult, FormData>(
    verifyMFAAction,
    {},
  );
  const isSetup = challenge.status === "mfa_setup_required";

  React.useEffect(() => {
    if (state.success && state.redirectTo) router.push(state.redirectTo);
  }, [state, router]);

  const groupedSecret = challenge.setupSecret?.match(/.{1,4}/g)?.join(" ");

  return (
    <div className="relative w-full max-w-[440px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[12rem] opacity-[0.04]">
        Safe
      </Watermark>
      <div className="relative overflow-hidden rounded-[28px] border border-[var(--border)] bg-[var(--surface)] p-5 shadow-[0_28px_80px_rgba(6,22,49,0.12)] sm:p-7">
        <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[var(--color-brand)] via-[var(--color-accent)] to-[var(--color-brand)]" />
        <PageHeader
          icon={<ShieldCheck className="size-7" />}
          title={isSetup ? "Protect your account" : "Verify it’s you"}
          description={
            isSetup
              ? "Privileged AuraEDU accounts require an authenticator app before access is granted."
              : "Enter the current code from your authenticator app to continue."
          }
        />

        {isSetup ? (
          <div className="mt-6 rounded-[var(--radius-lg)] border border-[var(--color-brand)]/20 bg-[var(--color-brand)]/[0.06] p-4">
            <p className="text-sm font-semibold text-[var(--foreground)]">
              Add AuraEDU to your authenticator
            </p>
            <ol className="mt-2 list-inside list-decimal space-y-1 text-sm leading-6 text-[var(--muted-foreground)]">
              <li>Open Google Authenticator, Microsoft Authenticator, or 1Password.</li>
              <li>Choose to enter a setup key.</li>
              <li>Use the key below, then enter the six-digit code it creates.</li>
            </ol>
            <div
              className="mt-3 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-3 text-center font-mono text-sm font-bold tracking-[0.14em] text-[var(--foreground)]"
              aria-label="Authenticator setup key"
            >
              {groupedSecret}
            </div>
          </div>
        ) : null}

        <form action={formAction} className="mt-6 space-y-4">
          <input type="hidden" name="challenge_token" value={challenge.challengeToken} />
          <input type="hidden" name="setup_secret" value={challenge.setupSecret ?? ""} />
          <input type="hidden" name="tenant" value={challenge.tenantCode} />
          <input type="hidden" name="next" value={challenge.nextPath} />
          <div>
            <label htmlFor="code" className="mb-1.5 block text-sm font-semibold">
              Six-digit code
            </label>
            <input
              id="code"
              name="code"
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              minLength={6}
              maxLength={6}
              autoFocus
              required
              placeholder="000000"
              className="h-14 w-full rounded-[var(--radius-md)] border border-border bg-[var(--input)] px-3.5 text-center font-mono text-2xl font-bold tracking-[0.35em] text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)]/50 focus-visible:border-[var(--color-brand)] focus-visible:bg-[var(--input-focus)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
            />
          </div>
          {state.error ? (
            <p
              role="alert"
              className="rounded-[var(--radius-sm)] bg-[var(--color-crit)]/10 px-3 py-2 text-sm text-[var(--color-crit)]"
            >
              {state.error}
            </p>
          ) : null}
          <Button type="submit" loading={pending} loadingLabel="Verifying" className="w-full">
            Verify and continue
          </Button>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="w-full text-center text-sm font-semibold text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
          >
            Start sign-in again
          </button>
        </form>
      </div>
      <p className="mt-4 text-center text-xs leading-5 text-[var(--muted-foreground)]">
        AuraEDU support will never ask for this code or setup key.
      </p>
    </div>
  );
}
