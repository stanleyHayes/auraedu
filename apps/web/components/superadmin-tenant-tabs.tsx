"use client";

import { usePathname } from "next/navigation";
import { PillNav } from "@auraedu/ui";

const TABS = [
  { slug: "", label: "Settings" },
  { slug: "students", label: "Students" },
  { slug: "staff", label: "Staff" },
  { slug: "finance", label: "Finance" },
  { slug: "attendance", label: "Attendance" },
  { slug: "audit", label: "Audit" },
  { slug: "delivery", label: "Delivery" },
] as const;

export interface SuperadminTenantTabsProps {
  tenantCode: string;
}

/** Sub-navigation for the tenant drill-down (read-only oversight views per school). */
export function SuperadminTenantTabs({ tenantCode }: SuperadminTenantTabsProps) {
  const pathname = usePathname();
  const base = `/superadmin/tenants/${tenantCode}`;

  return (
    <PillNav
      className="max-w-full flex-wrap"
      items={TABS.map((tab) => {
        const href = tab.slug ? `${base}/${tab.slug}` : base;
        return {
          id: tab.slug || "settings",
          label: tab.label,
          href,
          active: pathname === href,
        };
      })}
    />
  );
}
