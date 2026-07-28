import { Scale } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminGradingLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<Scale className="size-7" />}
        title="Grading scales"
        description="Loading grading scales…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
