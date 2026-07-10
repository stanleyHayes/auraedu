import { headers } from "next/headers";
import { LayoutDashboard } from "lucide-react";
import { PortalShell } from "@/components/portal-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(await headers()));
  return (
    <PortalShell
      tenant={tenant}
      page={{
        icon: <LayoutDashboard className="size-7" />,
        title: "Admin Console",
        description: "Manage students, staff, academics, and school settings.",
      }}
    >
      {children}
    </PortalShell>
  );
}
