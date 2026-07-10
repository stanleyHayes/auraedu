"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { AppSidebar, type NavGroup } from "@auraedu/ui";
import { AppTopbar } from "@/components/app-topbar";
import { DEFAULT_TENANT, TENANTS, type Tenant } from "@/lib/tenant";

const groups: NavGroup[] = [
  { heading: "People", items: [{ label: "Students", href: "/students" }, { label: "Staff", href: "/staff" }] },
  {
    heading: "Teaching",
    items: [
      { label: "Attendance", href: "/attendance" },
      { label: "Assessments", href: "/assessments" },
      { label: "Report cards", href: "/report-cards" },
    ],
  },
  { heading: "Money", items: [{ label: "Fees", href: "/fees", badge: 3 }] },
];

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
  const [tenant, setTenant] = React.useState<Tenant>(DEFAULT_TENANT);

  // Re-skin the whole app to the previewed school (accent only; chalkboard stays).
  React.useEffect(() => {
    document.documentElement.style.setProperty("--color-brand", tenant.brand);
  }, [tenant]);

  return (
    <div className="grid h-screen grid-cols-[240px_1fr] overflow-hidden max-md:grid-cols-1">
      <AppSidebar pathname={pathname} groups={groups} brand={<Brand short={tenant.short} />} className="h-full max-md:hidden" />
      <div className="flex min-w-0 flex-col overflow-hidden">
        <AppTopbar tenants={TENANTS} current={tenant} onSelect={setTenant} />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-4xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
