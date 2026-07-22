import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { AdminShell } from "@/components/admin-shell";
import { ADMIN_NAV, fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";
import { requireAuth, isAdmin } from "@/lib/auth";
import { checkRouteFeature } from "@/lib/features";
import { FeatureDisabled } from "@auraedu/flags";

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);
  const pathname = requestHeaders.get("x-pathname") ?? "";

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  const routeFeature = checkRouteFeature(pathname, tenant.features);
  const guardedChildren = routeFeature.enabled ? (
    children
  ) : (
    <FeatureDisabled feature={routeFeature.feature!} />
  );

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
      {guardedChildren}
    </AdminShell>
  );
}
