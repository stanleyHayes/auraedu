import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { Users } from "lucide-react";
import { ParentShell } from "@/components/parent-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, PARENT_NAV } from "@/lib/tenant";
import { requireAuth, isParent } from "@/lib/auth";

export default async function ParentLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  if (!session) {
    redirect("/login");
  }

  // Role guard: parents, school admins, and super admins may access the parent portal.
  if (!isParent(session)) {
    redirect("/login");
  }

  const user = {
    name: session.name ?? session.email ?? "Parent",
    email: session.email ?? "",
    role: session.role,
    initials: session.name
      ? session.name
          .split(" ")
          .map((n) => n[0])
          .join("")
          .slice(0, 2)
          .toUpperCase()
      : (session.email?.slice(0, 2).toUpperCase() ?? "P"),
  };

  return (
    <ParentShell
      tenant={tenant}
      navGroups={PARENT_NAV}
      featuresStub
      showMobileMenu
      user={user}
      page={{
        icon: <Users className="size-7" />,
        title: "Parent Portal",
        description: "Your children's attendance, results, fees, and announcements.",
      }}
    >
      {children}
    </ParentShell>
  );
}
