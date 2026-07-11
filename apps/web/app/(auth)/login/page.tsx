"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { KeyRound } from "lucide-react";
import { Button, PageHeader } from "@auraedu/ui";

function setCookie(name: string, value: string, days = 7) {
  try {
    const expires = new Date(Date.now() + days * 24 * 60 * 60 * 1000).toUTCString();
    document.cookie = `${name}=${encodeURIComponent(value)};path=/;expires=${expires};SameSite=Lax`;
  } catch {
    /* ignore */
  }
}

function buildDummyJwt(role: string, email: string, tenantId: string): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const payload = btoa(
    JSON.stringify({
      sub: "admin-user",
      tenant_id: tenantId,
      role,
      email,
      name: "School Administrator",
      perms: ["admin"],
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 60 * 60 * 24 * 7,
    }),
  );
  const signature = btoa("dummy-signature");
  return `${header}.${payload}.${signature}`;
}

export default function LoginPage() {
  const router = useRouter();
  const [loading, setLoading] = React.useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setLoading(true);

    const form = new FormData(e.currentTarget);
    const username = String((form.get("username") as string | null) ?? "admin");

    await new Promise((resolve) => setTimeout(resolve, 800));

    // TEMPORARY: until the Identity Service login flow is wired end-to-end, set a
    // dummy JWT + user cookie so the admin shell auth guard can demo the experience.
    const tenantId = "upshs";
    const token = buildDummyJwt("school_admin", `${username}@auraedu.local`, tenantId);
    setCookie("auraedu_access_token", token);
    setCookie(
      "auraedu_user",
      JSON.stringify({
        email: `${username}@auraedu.local`,
        name: "School Administrator",
        role: "school_admin",
      }),
    );

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
      <form onSubmit={(e) => void handleSubmit(e)} className="mt-8 space-y-4">
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
