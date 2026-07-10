import { headers } from "next/headers";
import { Shield } from "lucide-react";
import { PortalShell } from "@/components/portal-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

export default async function SuperAdminLayout({ children }: { children: React.ReactNode }) {
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(await headers()));
  return (
    <PortalShell
      tenant={tenant}
      page={{
        icon: <Shield className="size-7" />,
        title: "Super Admin",
        description: "Platform-wide tenant, feature-flag, and billing management.",
      }}
    >
      {children}
    </PortalShell>
  );
}
