import { getSession } from "@/lib/auth";
import { TeacherDashboard } from "@/components/teacher-dashboard";

export default async function TeacherDashboardPage() {
  const session = await getSession();
  return <TeacherDashboard userName={session?.name ?? session?.email ?? undefined} />;
}
