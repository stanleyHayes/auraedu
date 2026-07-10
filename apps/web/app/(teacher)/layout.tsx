import { headers } from "next/headers";
import { GraduationCap } from "lucide-react";
import { PortalShell } from "@/components/portal-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

export default async function TeacherLayout({ children }: { children: React.ReactNode }) {
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(await headers()));
  return (
    <PortalShell
      tenant={tenant}
      page={{
        icon: <GraduationCap className="size-7" />,
        title: "Teacher Portal",
        description: "Attendance, scores, assignments, and class insights.",
      }}
    >
      {children}
    </PortalShell>
  );
}
