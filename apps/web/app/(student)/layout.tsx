import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { BookOpen } from "lucide-react";
import { StudentShell } from "@/components/student-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, STUDENT_NAV } from "@/lib/tenant";
import { requireAuth, isStudent } from "@/lib/auth";
import { checkRouteFeature } from "@/lib/features";
import { FeatureDisabled } from "@auraedu/flags";

export default async function StudentLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);
  const pathname = requestHeaders.get("x-pathname") ?? "";

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  const routeFeature = checkRouteFeature(pathname, tenant.features);
  const guardedChildren = routeFeature.enabled ? children : (
    <FeatureDisabled feature={routeFeature.feature!} />
  );

  if (!session) {
    redirect("/login");
  }

  // Role guard: students, school admins, and super admins may access the student portal.
  if (!isStudent(session)) {
    redirect("/login");
  }

  const user = {
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
    <StudentShell
      tenant={tenant}
      navGroups={STUDENT_NAV}
      showMobileMenu
      user={user}
      page={{
        icon: <BookOpen className="size-7" />,
        title: "Student Portal",
        description: "Your timetable, assignments, results, and learning resources.",
      }}
    >
      {guardedChildren}
    </StudentShell>
  );
}
