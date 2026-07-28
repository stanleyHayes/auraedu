import { MonitorCheck } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminCbtLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title="CBT exams"
        description="Loading exams and submission traffic…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
