"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { AppSidebar, type NavGroup } from "@auraedu/ui";
import { AppTopbar } from "@/components/app-topbar";
import { DEFAULT_CODE, NAV, fetchTenant, type TenantData } from "@/lib/tenant";

function Brand({ short }: { short: string }) {
  return (
    <span className="flex items-center gap-2.5 font-display text-base font-extrabold tracking-tight">
      <span className="grid size-6 place-items-center rounded-md bg-foreground" aria-hidden="true">
        <svg viewBox="0 0 16 12" className="w-3 text-[var(--primary)]">
          <path d="M1 6.5 5.2 10.5 15 1" fill="none" stroke="currentColor" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
      {short}
    </span>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [code, setCode] = React.useState<string>(DEFAULT_CODE);
  const [data, setData] = React.useState<TenantData | null>(null);
  const [offline, setOffline] = React.useState(false);

  // Load branding + feature flags for the selected school from the Tenant Service,
  // and re-skin the app to its brand colour (accent only; chalkboard stays).
  React.useEffect(() => {
    let cancelled = false;
    setOffline(false);
    fetchTenant(code)
      .then((tenant) => {
        if (cancelled) return;
        setData(tenant);
        document.documentElement.style.setProperty("--color-brand", tenant.branding.brand.primary);
      })
      .catch(() => {
        if (!cancelled) setOffline(true);
      });
    return () => {
      cancelled = true;
    };
  }, [code]);

  // Feature-gate the nav from the live snapshot; before it loads, show everything.
  const enabled = data
    ? new Set(data.features.filter((f) => f.is_enabled).map((f) => f.feature_key))
    : null;
  const groups: NavGroup[] = NAV.map((group) => ({
    heading: group.heading,
    items: group.items
      .filter((item) => !item.feature || !enabled || enabled.has(item.feature))
      .map((item) => ({ label: item.label, href: item.href, badge: item.badge })),
  })).filter((group) => group.items.length > 0);

  return (
    <div className="grid h-screen grid-cols-[240px_1fr] overflow-hidden max-md:grid-cols-1">
      <AppSidebar
        pathname={pathname}
        groups={groups}
        brand={<Brand short={data?.short ?? "…"} />}
        className="h-full max-md:hidden"
      />
      <div className="flex min-w-0 flex-col overflow-hidden">
        <AppTopbar currentCode={code} onSelect={setCode} />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-4xl">
            {offline ? (
              <p className="mb-5 rounded-[var(--radius-md)] border border-border bg-[var(--accent)] px-4 py-2.5 text-sm text-[var(--foreground)]">
                Couldn&apos;t reach the Tenant Service — showing all modules. Start the backend
                with <code className="font-mono text-[var(--primary)]">make dev</code> (or set{" "}
                <code className="font-mono">TENANT_SERVICE_URL</code>).
              </p>
            ) : null}
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
