import { PageHeader, Reveal, Skeleton, StatsSkeleton } from "@auraedu/ui";
import { GitBranch } from "lucide-react";

export default function JourneysLoading() {
  return (
    <div className="space-y-7" aria-busy="true">
      <PageHeader
        icon={<GitBranch className="size-7" />}
        title="Communication journeys"
        description="Loading consent and delivery controls…"
      />
      <StatsSkeleton count={3} />
      <Reveal>
        <Skeleton className="h-[36rem] rounded-2xl" />
      </Reveal>
      <Skeleton className="h-72 rounded-2xl" />
    </div>
  );
}
