import { SuperadminTenantTabs } from "@/components/superadmin-tenant-tabs";

interface TenantDrilldownLayoutProps {
  children: React.ReactNode;
  params: Promise<{ code: string }>;
}

/**
 * Tenant drill-down shell: tab bar across the settings page and the read-only
 * oversight subpages (students, staff, finance, attendance, audit, delivery).
 */
export default async function TenantDrilldownLayout({
  children,
  params,
}: TenantDrilldownLayoutProps) {
  const { code } = await params;

  return (
    <div className="space-y-6">
      <SuperadminTenantTabs tenantCode={code} />
      {children}
    </div>
  );
}
