import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { SuperAdminShell } from "@/components/superadmin-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, SUPERADMIN_NAV } from "@/lib/tenant";
import { requireAuth, isSuperAdmin } from "@/lib/auth";

export default async function SuperAdminLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  if (!session) {
    redirect("/login");
  }

  if (!isSuperAdmin(session)) {
    redirect("/login");
  }

  const user = {
    id: session.user_id ?? session.sub,
    name: session.name ?? session.email ?? "Super Admin",
    email: session.email ?? "",
    role: session.role,
    initials: session.name
      ? session.name
          .split(" ")
          .map((n) => n[0])
          .join("")
          .slice(0, 2)
          .toUpperCase()
      : (session.email?.slice(0, 2).toUpperCase() ?? "S"),
  };

  return (
    <SuperAdminShell tenant={tenant} navGroups={SUPERADMIN_NAV} showMobileMenu user={user}>
      {children}
    </SuperAdminShell>
  );
}
