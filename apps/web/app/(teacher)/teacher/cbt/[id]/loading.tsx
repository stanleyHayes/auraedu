import { MonitorCheck } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function TeacherCbtExamLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<MonitorCheck className="size-7" />}
        title="Exam detail"
        description="Loading the exam, its questions, and submissions…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
