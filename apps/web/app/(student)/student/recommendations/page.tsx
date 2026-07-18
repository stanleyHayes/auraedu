import { Sparkles } from "lucide-react";
import { EmptyState } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient, getSession } from "@/lib/api";

type Recommendation = OpenAPI.ai_recommendation_v1.components["schemas"]["Recommendation"];

export default async function StudentRecommendationsPage() {
  const session = await getSession();
  let recommendations: Recommendation[] = [];
  let error: string | null = null;

  // student_id is required by the contract; without a session there is nothing
  // to query, so fall through to the empty state.
  if (session?.user_id) {
    try {
      const client = await createServerClient();
      // Students only ever see approved recommendations. NOTE: student_id is the
      // identity user id until the backend exposes an actor→student-record mapping.
      const list = await client.get<
        OpenAPI.ai_recommendation_v1.components["schemas"]["RecommendationList"]
      >(`/api/v1/ai/recommendations?student_id=${session.user_id}&status=approved`);
      recommendations = list.data ?? [];
    } catch {
      error = "Could not load recommendations right now.";
    }
  }

  if (error) {
    return (
      <EmptyState
        icon={<Sparkles className="size-8" />}
        title="Recommendations unavailable"
        description={error}
      />
    );
  }

  if (recommendations.length === 0) {
    return (
      <EmptyState
        icon={<Sparkles className="size-8" />}
        title="No recommendations yet"
        description="Personalised learning recommendations will appear here once your teachers approve them."
      />
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="font-heading text-lg font-semibold tracking-tight">AI recommendations</h2>
      <ul className="space-y-3">
        {recommendations.map((recommendation) => (
          <li
            key={recommendation.id}
            className="rounded-[var(--radius-md)] border border-[var(--border)] bg-[var(--surface)] p-4"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <h3 className="font-medium text-[var(--foreground)]">{recommendation.title}</h3>
                {recommendation.description ? (
                  <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                    {recommendation.description}
                  </p>
                ) : null}
                <p className="mt-1 text-xs capitalize text-[var(--muted-foreground)]">
                  {recommendation.recommendation_type.replace(/_/g, " ")}
                </p>
              </div>
              <span className="shrink-0 text-xs text-[var(--muted-foreground)]">
                {Math.round(recommendation.confidence * 100)}% confidence
              </span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
