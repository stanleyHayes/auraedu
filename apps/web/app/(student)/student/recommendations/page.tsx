import { Sparkles } from "lucide-react";
import { EmptyState } from "@auraedu/ui";

export default function StudentRecommendationsPage() {
  return (
    <EmptyState
      icon={<Sparkles className="size-8" />}
      title="AI recommendations"
      description="Personalised learning recommendations will appear here once AI insights are enabled."
    />
  );
}
