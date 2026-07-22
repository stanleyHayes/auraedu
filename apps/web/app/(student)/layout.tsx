import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { StudentShell } from "@/components/student-shell";
import { RouteFeatureGuard } from "@/components/route-feature-guard";
import { fetchTenantBranding, getTenantCodeFromHeaders, STUDENT_NAV } from "@/lib/tenant";
import { requireAuth, isStudent } from "@/lib/auth";

export default async function StudentLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  if (!session) {
    redirect("/login");
  }

  // Role guard: students, school admins, and super admins may access the student portal.
  if (!isStudent(session)) {
    redirect("/login");
  }

  const user = {
    id: session.user_id ?? session.sub,
    name: session.name ?? session.email ?? "Student",
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
    <StudentShell tenant={tenant} navGroups={STUDENT_NAV} showMobileMenu user={user}>
      <RouteFeatureGuard flags={tenant.features}>{children}</RouteFeatureGuard>
    </StudentShell>
  );
}
