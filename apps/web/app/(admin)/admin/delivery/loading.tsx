import { SendHorizonal } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminDeliveryLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<SendHorizonal className="size-7" />}
        title="Delivery logs"
        description="Loading notification delivery outcomes…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-20 rounded-2xl" />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
