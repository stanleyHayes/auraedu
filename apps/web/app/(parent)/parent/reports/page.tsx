import { FileText, ClipboardList } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";

export default function ParentReportsPage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<FileText className="size-6" />}
        title="Report Cards"
        description="Download and view your children's report cards."
      />
      <EmptyState
        icon={<ClipboardList className="size-8" />}
        title="No report cards"
        description="Report cards will appear here once they are published by the school."
      />
    </div>
  );
}
