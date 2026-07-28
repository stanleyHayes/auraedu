import { TrendingUp } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminAiPredictionsLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<TrendingUp className="size-7" />}
        title="AI predictions"
        description="Loading the prediction pipeline…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-20 rounded-2xl" />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
