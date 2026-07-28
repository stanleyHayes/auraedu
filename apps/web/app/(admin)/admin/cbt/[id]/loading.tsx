import { MonitorCheck } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminCbtExamLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title="Exam detail"
        description="Loading exam and submissions…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
