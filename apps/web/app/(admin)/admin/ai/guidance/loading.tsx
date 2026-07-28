import { Compass } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminAiGuidanceLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<Compass className="size-7" />}
        title="Career guidance"
        description="Loading the guidance pipeline…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-20 rounded-2xl" />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
