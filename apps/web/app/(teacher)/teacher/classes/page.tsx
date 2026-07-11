import { Users } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";

export default function TeacherClassesPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Users className="size-6" />}
        title="My Classes"
        description="Classes and subjects assigned to you."
      />
      <EmptyState
        icon={<Users className="size-8" />}
        title="Classes coming soon"
        description="Your class list will appear here once the academic management service is wired."
      />
    </div>
  );
}
