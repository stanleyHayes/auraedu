import { Sparkles } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminAiRecommendationsLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<Sparkles className="size-7" />}
        title="AI recommendations"
        description="Loading the recommendation pipeline…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-20 rounded-2xl" />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
