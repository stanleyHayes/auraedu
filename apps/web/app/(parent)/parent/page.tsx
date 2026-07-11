import { getSession } from "@/lib/auth";
import { ParentDashboard } from "@/components/parent-dashboard";

export default async function ParentDashboardPage() {
  const session = await getSession();
  return <ParentDashboard userName={session?.name ?? session?.email ?? undefined} />;
}
