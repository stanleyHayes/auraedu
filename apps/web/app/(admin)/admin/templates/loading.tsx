import { MailCheck } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminTemplatesLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<MailCheck className="size-7" />}
        title="Message templates"
        description="Loading templates…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
