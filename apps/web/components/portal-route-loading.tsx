import { CardGridSkeleton, PageHeaderSkeleton, StatsSkeleton } from "@auraedu/ui";

export function PortalRouteLoading() {
  return (
    <div aria-busy="true" aria-label="Loading workspace" className="space-y-7" role="status">
      <span className="sr-only">Loading your workspace…</span>
      <PageHeaderSkeleton />
      <StatsSkeleton />
      <CardGridSkeleton />
    </div>
  );
}
