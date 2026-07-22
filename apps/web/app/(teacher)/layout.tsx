import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { TeacherShell } from "@/components/teacher-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, TEACHER_NAV } from "@/lib/tenant";
import { requireAuth, isTeacher } from "@/lib/auth";
import { checkRouteFeature } from "@/lib/features";
import { FeatureDisabled } from "@auraedu/flags";

export default async function TeacherLayout({ children }: { children: React.ReactNode }) {
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

  if (!isTeacher(session)) {
    redirect("/login");
  }

  const user = {
    id: session.user_id ?? session.sub,
    name: session.name ?? session.email ?? "Teacher",
    email: session.email ?? "",
    role: session.role,
    initials: session.name
      ? session.name
          .split(" ")
          .map((n) => n[0])
          .join("")
          .slice(0, 2)
          .toUpperCase()
      : (session.email?.slice(0, 2).toUpperCase() ?? "T"),
  };

  return (
    <TeacherShell tenant={tenant} navGroups={TEACHER_NAV} user={user} showMobileMenu>
      {guardedChildren}
    </TeacherShell>
  );
}
