import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { GraduationCap } from "lucide-react";
import { TeacherShell } from "@/components/teacher-shell";
import { fetchTenantBranding, getTenantCodeFromHeaders, TEACHER_NAV } from "@/lib/tenant";
import { requireAuth, isTeacher } from "@/lib/auth";

export default async function TeacherLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const tenantCode = getTenantCodeFromHeaders(requestHeaders);

  const [tenant, session] = await Promise.all([
    fetchTenantBranding(tenantCode),
    requireAuth().catch(() => null),
  ]);

  if (!session) {
    redirect("/login");
  }

  if (!isTeacher(session)) {
    redirect("/login");
  }

  const user = {
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
    <TeacherShell
      tenant={tenant}
      navGroups={TEACHER_NAV}
      user={user}
      showMobileMenu
      page={{
        icon: <GraduationCap className="size-7" />,
        title: "Teacher Portal",
        description: "Attendance, scores, assignments, and class insights.",
      }}
    >
      {children}
    </TeacherShell>
  );
}
