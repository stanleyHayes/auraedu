import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { ParentShell } from "@/components/parent-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, PARENT_NAV } from "@/lib/tenant";
import { requireAuth, isParent } from "@/lib/auth";
import { checkRouteFeature } from "@/lib/features";
import { FeatureDisabled } from "@auraedu/flags";

export default async function ParentLayout({ children }: { children: React.ReactNode }) {
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

  // Role guard: parents, school admins, and super admins may access the parent portal.
  if (!isParent(session)) {
    redirect("/login");
  }

  const user = {
    id: session.user_id ?? session.sub,
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
    <ParentShell tenant={tenant} navGroups={PARENT_NAV} showMobileMenu user={user}>
      {guardedChildren}
    </ParentShell>
  );
}
