"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { AppSidebar, type NavGroup, PageHeader, type PageHeaderProps } from "@auraedu/ui";
import { AppTopbar } from "@/components/app-topbar";
import { NAV, type TenantData, type NavGroupDef } from "@/lib/tenant";
import { useFeatureSnapshot } from "@auraedu/flags";

function Brand({ tenant }: { tenant?: TenantData }) {
  return (
    <span className="flex items-center gap-2.5 font-display text-base font-extrabold tracking-tight">
      <span className="grid size-6 place-items-center rounded-md bg-foreground" aria-hidden="true">
        <svg viewBox="0 0 16 12" className="w-3 text-[var(--primary)]">
          <path
            d="M1 6.5 5.2 10.5 15 1"
            fill="none"
            stroke="currentColor"
            strokeWidth={2.4}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      {tenant?.short ?? "AuraEDU"}
    </span>
  );
}

function filterNav(groups: NavGroupDef[], enabled: Set<string> | null): NavGroup[] {
  return groups
    .map((group) => ({
      heading: group.heading,
      items: group.items
        .filter((item) => !item.feature || !enabled || enabled.has(item.feature))
        .map((item) => ({ label: item.label, href: item.href, badge: item.badge })),
    }))
    .filter((group) => group.items.length > 0);
}

export interface PortalShellProps {
  tenant?: TenantData;
  page?: PageHeaderProps;
  children: React.ReactNode;
}

export function PortalShell({ tenant, page, children }: PortalShellProps) {
  const pathname = usePathname();
  const snapshot = useFeatureSnapshot();
  const enabled = snapshot
    ? new Set(snapshot.flags.filter((f) => f.is_enabled).map((f) => f.feature_key))
    : null;
  const groups = filterNav(NAV, enabled);

  return (
    <div className="grid h-screen grid-cols-[240px_1fr] overflow-hidden max-md:grid-cols-1">
      <AppSidebar pathname={pathname} groups={groups} brand={<Brand tenant={tenant} />} className="h-full max-md:hidden" />
      <div className="flex min-w-0 flex-col overflow-hidden">
        <AppTopbar currentCode={tenant?.code} />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-4xl">
            {page ? (
              <div className="mb-6">
                <PageHeader {...page} />
              </div>
            ) : null}
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
