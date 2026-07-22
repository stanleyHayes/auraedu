import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin-shell";
import { RouteFeatureGuard } from "@/components/route-feature-guard";
import { ADMIN_NAV, fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";
import { requireAuth, isAdmin } from "@/lib/auth";

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  if (!session) {
    redirect("/login");
  }

  // Role guard: school admins and platform super admins may access the console.
  // If role checks are not wired, requireAuth already enforces a valid JWT.
  if (!isAdmin(session)) {
    redirect("/login");
  }

  const user = {
    id: session.user_id ?? session.sub,
    name: session.name ?? session.email ?? "Admin",
    email: session.email ?? "",
    role: session.role,
    initials: session.name
      ? session.name
          .split(" ")
          .map((n) => n[0])
          .join("")
          .slice(0, 2)
          .toUpperCase()
      : (session.email?.slice(0, 2).toUpperCase() ?? "A"),
  };

  return (
    <AdminShell tenant={tenant} navGroups={ADMIN_NAV} showMobileMenu user={user}>
      <RouteFeatureGuard flags={tenant.features}>{children}</RouteFeatureGuard>
    </AdminShell>
  );
}
