import { Megaphone, BellOff } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";

export default function ParentNotificationsPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Megaphone className="size-6" />}
        title="Notifications"
        description="School announcements and notices for parents."
      />
      <EmptyState
        icon={<BellOff className="size-8" />}
        title="No notifications"
        description="School announcements and notices will appear here."
      />
    </div>
  );
}
