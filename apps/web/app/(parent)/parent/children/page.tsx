import { Baby, UserPlus } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";

export default function ParentChildrenPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Baby className="size-6" />}
        title="My Children"
        description="View and manage your children's school profiles."
      />
      <EmptyState
        icon={<UserPlus className="size-8" />}
        title="No children linked yet"
        description="Your children's profiles will appear here once they are associated with your account."
      />
    </div>
  );
}
