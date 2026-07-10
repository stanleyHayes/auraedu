"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { KeyRound } from "lucide-react";
import { Button, PageHeader } from "@auraedu/ui";

export default function LoginPage() {
  const router = useRouter();
  const [loading, setLoading] = React.useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setLoading(true);
    await new Promise((resolve) => setTimeout(resolve, 800));
    setLoading(false);
    router.push("/admin");
  }

  return (
    <>
      <PageHeader
        icon={<KeyRound className="size-7" />}
        title="Welcome back"
        description="Sign in with the username and password from your school administrator."
      />
      <form onSubmit={handleSubmit} className="mt-8 space-y-4">
        <div>
          <label htmlFor="username" className="mb-1.5 block text-sm font-medium">
            Username
          </label>
          <input
            id="username"
            name="username"
            type="text"
            autoComplete="username"
            required
            className="h-10 w-full rounded-[var(--radius-sm)] border border-border bg-surface px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
          />
        </div>
        <div>
          <label htmlFor="password" className="mb-1.5 block text-sm font-medium">
            Password
          </label>
          <input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            required
            className="h-10 w-full rounded-[var(--radius-sm)] border border-border bg-surface px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
          />
        </div>
        <Button type="submit" loading={loading} loadingLabel="Signing in" className="w-full">
          Sign in
        </Button>
        <p className="text-center text-xs text-muted-foreground">
          No public sign-up — contact your school administrator for an account.
        </p>
      </form>
    </>
  );
}
