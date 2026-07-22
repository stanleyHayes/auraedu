"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import {
  AppSidebar,
  Sheet,
  type NavGroup,
  PageHeader,
  PageGuideProvider,
  type PageHeaderProps,
  Watermark,
} from "@auraedu/ui";
import { AppTopbar } from "@/components/app-topbar";
import { AuraEduLogo } from "@/components/auraedu-logo";
import { NAV, type TenantData, type NavGroupDef } from "@/lib/tenant";
import { enabledFeatureKeys, isNavigationFeatureVisible } from "@/lib/features";
import { useFeatureSnapshot } from "@auraedu/flags";
import { AppTour } from "@/components/app-tour";
import { getPageGuide } from "@/lib/page-guides";

function Brand({ tenant, className }: { tenant?: TenantData; className?: string }) {
  return (
    <span className={cn("flex min-w-0 items-center gap-3", className)}>
      <span className="grid size-11 shrink-0 place-items-center rounded-2xl border border-white/10 bg-white/[0.07] shadow-[inset_0_1px_0_rgba(255,255,255,0.1)]">
        <AuraEduLogo tone="light" variant="mark" className="size-7" />
      </span>
      <span className="min-w-0">
        <AuraEduLogo tone="light" className="h-5" />
        <span className="mt-1 block truncate font-mono text-[9px] font-bold uppercase tracking-[0.16em] text-white/45">
          {tenant?.short ?? "Education OS"}
        </span>
      </span>
    </span>
  );
}

function filterNav(groups: NavGroupDef[], enabled: Set<string> | null, stub: boolean): NavGroup[] {
  return groups
    .map((group) => ({
      heading: group.heading,
      items: group.items
        .filter((item) => isNavigationFeatureVisible(item.feature, enabled, stub))
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
    id?: string;
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
  const enabled = snapshot ? enabledFeatureKeys(snapshot.flags) : null;
  const groups = filterNav(navGroups, enabled, featuresStub);
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const role = user?.role ?? "member";
  const roleLabel =
    role === "superadmin" || role === "platform_super_admin"
      ? "Platform operations"
      : role === "admin" || role === "school_admin"
        ? "School command centre"
        : role === "teacher"
          ? "Teaching workspace"
          : role === "student"
            ? "Learning workspace"
            : role === "parent"
              ? "Family workspace"
              : role === "applicant"
                ? "Admissions journey"
                : "Education workspace";
  const roleHome =
    role === "platform_super_admin" || role === "superadmin"
      ? "/superadmin"
      : role === "school_admin" || role === "admin"
        ? "/admin"
        : `/${role}`;
  const resolveGuide = React.useCallback(
    (title: string, description?: string) => getPageGuide(pathname, { title, description }),
    [pathname],
  );

  return (
    <div
      className="portal-frame grid h-screen grid-cols-[288px_1fr] overflow-hidden max-md:grid-cols-1"
      data-role={role}
    >
      <a className="app-skip-link" href="#portal-main">
        Skip to page content
      </a>
      <div data-tour="desktop-navigation" className="max-md:hidden">
        <AppSidebar
          pathname={pathname}
          groups={groups}
          brand={<Brand tenant={tenant} />}
          workspaceLabel={roleLabel}
          footer={
            <div className="flex items-center gap-2 text-[11px] text-white/55">
              <span className="relative flex size-2">
                <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-300 opacity-50 motion-reduce:animate-none" />
                <span className="relative inline-flex size-2 rounded-full bg-emerald-300" />
              </span>
              Systems connected
            </div>
          }
          className="h-full"
        />
      </div>
      <Sheet
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        side="left"
        className="w-72 bg-[var(--color-navy)] p-0"
        closeButtonClassName="text-[var(--color-cream)] hover:bg-white/10"
      >
        <div className="flex h-full flex-col">
          <div className="px-4 pb-3 pt-4">
            <Brand tenant={tenant} />
          </div>
          <AppSidebar
            pathname={pathname}
            groups={groups}
            brand={null}
            workspaceLabel={roleLabel}
            onNavigate={() => setMobileOpen(false)}
            className="flex-1 border-0"
          />
        </div>
      </Sheet>
      <div className="portal-workspace relative flex min-w-0 flex-col overflow-hidden">
        {/* ambient watermark behind scrollable content */}
        <Watermark className="pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 text-[18rem] opacity-[0.025]">
          Aura
        </Watermark>
        <AppTopbar
          currentCode={tenant?.code}
          user={user}
          workspaceLabel={roleLabel}
          onLogout={onLogout}
          onMobileMenuToggle={() => setMobileOpen(true)}
          showMobileMenu={showMobileMenu}
        />
        <PageGuideProvider resolve={resolveGuide}>
          <main
            id="portal-main"
            className="portal-main relative z-10 flex-1 overflow-y-auto px-4 py-5 sm:px-6 sm:py-7 lg:px-8"
            tabIndex={-1}
          >
            <div className="mx-auto max-w-[1440px]">
              {page ? (
                <div className="mb-6">
                  <PageHeader {...page} />
                </div>
              ) : null}
              {children}
            </div>
          </main>
        </PageGuideProvider>
      </div>
      <AppTour
        tenantId={tenant?.code ?? "platform"}
        userId={user?.id ?? user?.email ?? "member"}
        mode={role}
        autoStart={pathname === roleHome}
      />
    </div>
  );
}

function cn(...inputs: (string | false | undefined)[]) {
  return inputs.filter(Boolean).join(" ");
}
