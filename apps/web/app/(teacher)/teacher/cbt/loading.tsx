import { MonitorCheck } from "lucide-react";
import { PageHeader, Skeleton } from "@auraedu/ui";

export default function TeacherCbtLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<MonitorCheck className="size-6" />}
        title="CBT exams"
        description="Loading your exams…"
      />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
