import { headers } from "next/headers";
import { Users } from "lucide-react";
import { PortalShell } from "@/components/portal-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

export default async function ParentLayout({ children }: { children: React.ReactNode }) {
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(await headers()));
  return (
    <PortalShell
      tenant={tenant}
      page={{
        icon: <Users className="size-7" />,
        title: "Parent Portal",
        description: "Your children's attendance, results, fees, and announcements.",
      }}
    >
      {children}
    </PortalShell>
  );
}
