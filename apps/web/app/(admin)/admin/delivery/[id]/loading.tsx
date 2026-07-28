import { SendHorizonal } from "lucide-react";
import { PageHeader, Skeleton } from "@auraedu/ui";

export default function AdminDeliveryDetailLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<SendHorizonal className="size-7" />}
        title="Message detail"
        description="Loading message and status history…"
      />
      <div className="grid gap-4 lg:grid-cols-3">
        <Skeleton className="h-80 rounded-2xl lg:col-span-2" />
        <Skeleton className="h-80 rounded-2xl" />
      </div>
    </div>
  );
}
