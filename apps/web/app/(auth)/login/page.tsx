"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { KeyRound } from "lucide-react";
import { Button, PageHeader, Watermark } from "@auraedu/ui";
import { loginAction, type LoginResult } from "./actions";

export default function LoginPage() {
  const router = useRouter();
  const [state, formAction, pending] = React.useActionState<LoginResult, FormData>(loginAction, {});

  React.useEffect(() => {
    if (state.success && state.redirectTo) {
      router.push(state.redirectTo);
    }
  }, [state, router]);

  return (
    <div className="relative w-full max-w-[420px]">
      <Watermark className="pointer-events-none absolute -right-20 -top-28 text-[12rem] opacity-[0.04]">
        Aura
      </Watermark>
      <div className="relative rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] p-6 shadow-lg">
        <PageHeader
          icon={<KeyRound className="size-7" />}
          title="Welcome back"
          description="Sign in with the email and password from your school administrator."
        />
        <form action={formAction} className="mt-8 space-y-4">
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
              className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--color-parchment)] px-3.5 text-sm text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:border-[var(--color-gold)] focus-visible:bg-[var(--surface)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
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
              className="h-11 w-full rounded-[var(--radius-md)] border border-border bg-[var(--color-parchment)] px-3.5 text-sm text-[var(--foreground)] shadow-sm placeholder:text-[var(--muted-foreground)] focus-visible:border-[var(--color-gold)] focus-visible:bg-[var(--surface)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]/40"
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
          <p className="text-center text-xs text-muted-foreground">
            No public sign-up — contact your school administrator for an account.
          </p>
        </form>
      </div>
    </div>
  );
}
