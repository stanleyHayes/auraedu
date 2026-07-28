import { UsersRound } from "lucide-react";
import { PageHeader, Skeleton, StatsSkeleton } from "@auraedu/ui";

export default function AdminUsersLoading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <PageHeader
        icon={<UsersRound className="size-7" />}
        title="Users & roles"
        description="Loading identity accounts…"
      />
      <StatsSkeleton count={3} />
      <Skeleton className="h-96 rounded-2xl" />
    </div>
  );
}
