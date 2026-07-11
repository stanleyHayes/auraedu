import { FileText } from "lucide-react";
import { EmptyState } from "@auraedu/ui";

export default function StudentReportCardPage() {
  return (
    <EmptyState
      icon={<FileText className="size-8" />}
      title="Report card"
      description="Your term report cards will appear here once they are published."
    />
  );
}
