import { headers } from "next/headers";
import { BookOpen } from "lucide-react";
import { PortalShell } from "@/components/portal-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";

export default async function StudentLayout({ children }: { children: React.ReactNode }) {
  const tenant = await fetchTenantBranding(getTenantCodeFromHeaders(await headers()));
  return (
    <PortalShell
      tenant={tenant}
      page={{
        icon: <BookOpen className="size-7" />,
        title: "Student Portal",
        description: "Your timetable, assignments, results, and resources.",
      }}
    >
      {children}
    </PortalShell>
  );
}
