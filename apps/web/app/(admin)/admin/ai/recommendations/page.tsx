import { Sparkles } from "lucide-react";
import { PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminAiOversightList, type AiOversightQuery } from "@/components/admin-ai-oversight";
import type { AiOversightItem } from "@/lib/admin-ai";

type Recommendation = OpenAPI.ai_recommendation_v1.components["schemas"]["Recommendation"];
type RecommendationList = OpenAPI.ai_recommendation_v1.components["schemas"]["RecommendationList"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

const STATUS_OPTIONS = ["pending", "approved", "rejected", "overridden"];

export default async function AdminAiRecommendationsPage({
  searchParams,
}: {
  searchParams: Promise<AiOversightQuery>;
}) {
  await requireAuth();
  const query = await searchParams;

  const params = new URLSearchParams();
  params.set("limit", "50");
  if (query.student_id?.trim()) params.set("student_id", query.student_id.trim());
  if (query.status?.trim()) params.set("status", query.status.trim());
  if (query.cursor?.trim()) params.set("cursor", query.cursor.trim());

  let items: AiOversightItem[] = [];
  let studentNames = new Map<string, string>();
  let nextCursor: string | null = null;
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [listResult, studentResult] = await Promise.allSettled([
      client.get<RecommendationList>(`/api/v1/ai/recommendations/recommendations?${params}`),
      client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        "/api/v1/students?limit=100",
      ),
    ]);
    if (listResult.status === "fulfilled") {
      const rows = listResult.value.data ?? [];
      items = rows.map((row: Recommendation) => ({
        id: row.id,
        student_id: row.student_id,
        itemType: row.recommendation_type,
        title: row.title,
        detail: row.description ?? null,
        status: row.status,
        confidence: row.confidence,
        explanation: row.explanation ?? null,
        created_at: row.created_at,
      }));
      nextCursor = listResult.value.next_cursor ?? null;
    } else {
      error =
        listResult.reason instanceof Error
          ? listResult.reason.message
          : "Failed to load recommendations";
    }
    if (studentResult.status === "fulfilled") {
      const students: Student[] = studentResult.value.data ?? [];
      studentNames = new Map(
        students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
      );
    }
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load recommendations";
  }

  // Honest filtering even where the service does not honour the status parameter yet.
  const visible = query.status?.trim()
    ? items.filter((item) => item.status === query.status?.trim())
    : items;
  const pending = visible.filter((item) => item.status === "pending").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Sparkles className="size-7" />}
        title="AI recommendations"
        description="Every generated recommendation — including items still awaiting staff review. Learners only see approved items; this console sees the full pipeline."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Visible items" value={visible.length} unit="records" />
        <StatCard label="Awaiting review" value={pending} tone={pending > 0 ? "warn" : "default"} />
        <StatCard
          label="Approved"
          value={visible.filter((item) => item.status === "approved").length}
          tone="ok"
        />
      </div>
      <AdminAiOversightList
        basePath="/admin/ai/recommendations"
        noun="recommendations"
        items={visible}
        studentNames={studentNames}
        statusOptions={STATUS_OPTIONS}
        query={query}
        nextCursor={nextCursor}
        error={error}
        emptyTitle="No recommendations yet"
        emptyDescription="Recommendations generated for learners will appear here for review before learners ever see them."
      />
    </div>
  );
}
