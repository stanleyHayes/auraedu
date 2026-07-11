import { getSession } from "@/lib/auth";
import { StudentDashboard } from "@/components/student-dashboard";

export default async function StudentDashboardPage() {
  const session = await getSession();
  return <StudentDashboard userName={session?.name ?? session?.email ?? undefined} />;
}
