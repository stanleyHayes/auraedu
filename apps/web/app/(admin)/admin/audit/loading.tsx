import { ScrollText } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminAuditLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<ScrollText className="size-7" />}
        title="Audit log"
        description="Loading the school's activity trail…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-24 rounded-2xl" />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
