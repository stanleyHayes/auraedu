import { redirect } from "next/navigation";
import { SuperAdminShell } from "@/components/superadmin-shell";
import { SUPERADMIN_NAV } from "@/lib/tenant";
import { requireAuth, isSuperAdmin } from "@/lib/auth";

export default async function SuperAdminLayout({ children }: { children: React.ReactNode }) {
  const session = await requireAuth().catch(() => null);

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
    <SuperAdminShell navGroups={SUPERADMIN_NAV} showMobileMenu user={user}>
      {children}
    </SuperAdminShell>
  );
}
