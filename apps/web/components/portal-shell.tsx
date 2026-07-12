"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { AppSidebar, Sheet, type NavGroup, PageHeader, type PageHeaderProps, Watermark } from "@auraedu/ui";
import { AppTopbar } from "@/components/app-topbar";
import { NAV, type TenantData, type NavGroupDef } from "@/lib/tenant";
import { useFeatureSnapshot } from "@auraedu/flags";

function Brand({ tenant, className }: { tenant?: TenantData; className?: string }) {
  return (
    <span
      className={cn(
        "flex items-center gap-2.5 font-sans text-base font-extrabold tracking-tight text-[var(--color-cream)]",
        className,
      )}
    >
      <span
        className="grid size-7 place-items-center rounded-md bg-gradient-to-br from-[var(--color-gold)] to-[var(--color-gold-soft)] text-[var(--color-navy)] shadow-sm"
        aria-hidden="true"
      >
        <svg viewBox="0 0 16 12" className="w-3">
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

function filterNav(groups: NavGroupDef[], enabled: Set<string> | null, stub: boolean): NavGroup[] {
  return groups
    .map((group) => ({
      heading: group.heading,
      items: group.items
        .filter((item) => !item.feature || !enabled || stub || enabled.has(item.feature))
        .map((item) => ({ label: item.label, href: item.href, badge: item.badge })),
    }))
    .filter((group) => group.items.length > 0);
}

export interface PortalShellProps {
  tenant?: TenantData;
  page?: PageHeaderProps;
  children: React.ReactNode;
  navGroups?: NavGroupDef[];
  featuresStub?: boolean;
  user?: {
    name?: string;
    email?: string;
    role?: string;
    initials?: string;
  };
  onLogout?: () => void;
  showMobileMenu?: boolean;
}

export function PortalShell({
  tenant,
  page,
  children,
  navGroups = NAV,
  featuresStub = false,
  user,
  onLogout,
  showMobileMenu = false,
}: PortalShellProps) {
  const pathname = usePathname();
  const snapshot = useFeatureSnapshot();
  const enabled = snapshot
    ? new Set(snapshot.flags.filter((f) => f.is_enabled).map((f) => f.feature_key))
    : null;
  const groups = filterNav(navGroups, enabled, featuresStub);
  const [mobileOpen, setMobileOpen] = React.useState(false);

  return (
    <div className="grid h-screen grid-cols-[240px_1fr] overflow-hidden max-md:grid-cols-1">
      <div data-tour="desktop-navigation" className="max-md:hidden">
        <AppSidebar
          pathname={pathname}
          groups={groups}
          brand={<Brand tenant={tenant} />}
          className="h-full"
        />
      </div>
      <Sheet open={mobileOpen} onClose={() => setMobileOpen(false)} side="left" className="w-60 bg-[var(--color-navy)] p-0" closeButtonClassName="text-[var(--color-cream)] hover:bg-white/10">
        <div className="flex h-full flex-col">
          <div className="px-4 pb-3 pt-4">
            <Brand tenant={tenant} />
          </div>
          <AppSidebar
            pathname={pathname}
            groups={groups}
            brand={null}
            onNavigate={() => setMobileOpen(false)}
            className="flex-1 border-0"
          />
        </div>
      </Sheet>
      <div className="relative flex min-w-0 flex-col overflow-hidden">
        {/* ambient watermark behind scrollable content */}
        <Watermark className="pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 text-[18rem] opacity-[0.025]">
          Aura
        </Watermark>
        <AppTopbar
          currentCode={tenant?.code}
          user={user}
          onLogout={onLogout}
          onMobileMenuToggle={() => setMobileOpen(true)}
          showMobileMenu={showMobileMenu}
        />
        <main className="relative z-10 flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-5xl">
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

function cn(...inputs: (string | false | undefined)[]) {
  return inputs.filter(Boolean).join(" ");
}
