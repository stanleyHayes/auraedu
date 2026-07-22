import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { StudentShell } from "@/components/student-shell";
import { RouteFeatureGuard } from "@/components/route-feature-guard";
import { APPLICANT_NAV, fetchTenantBranding, getTenantCodeFromHeaders } from "@/lib/tenant";
import { isApplicant, requireAuth } from "@/lib/auth";
export default async function ApplicantLayout({ children }: { children: React.ReactNode }) {
  const requestHeaders = await headers();
  const code = getTenantCodeFromHeaders(requestHeaders);
  const [tenant, session] = await Promise.all([
    fetchTenantBranding(code),
    requireAuth().catch(() => null),
  ]);
  if (!session || !isApplicant(session)) redirect("/login");
  return (
    <StudentShell
      tenant={tenant}
      navGroups={APPLICANT_NAV}
      showMobileMenu
      user={{
        id: session.user_id ?? session.sub,
        name: session.name ?? session.email ?? "Applicant",
        email: session.email ?? "",
        role: session.role,
        initials: (session.name ?? "AP")
          .split(" ")
          .map((part) => part[0])
          .join("")
          .slice(0, 2)
          .toUpperCase(),
      }}
    >
      <RouteFeatureGuard flags={tenant.features}>{children}</RouteFeatureGuard>
    </StudentShell>
  );
}
