import { Compass, MapPinned } from "lucide-react";
import { PageHeader, EmptyState } from "@auraedu/ui";

export default function ParentGuidancePage() {
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Compass className="size-6" />}
        title="Guidance"
        description="Career guidance and counselling updates for your children."
      />
      <EmptyState
        icon={<MapPinned className="size-8" />}
        title="No guidance updates"
        description="Career guidance notes and recommendations will appear here."
      />
    </div>
  );
}
