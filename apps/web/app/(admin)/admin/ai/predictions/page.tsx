import { TrendingUp } from "lucide-react";
import { PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminAiOversightList, type AiOversightQuery } from "@/components/admin-ai-oversight";
import type { AiOversightItem } from "@/lib/admin-ai";

type Prediction = OpenAPI.ai_prediction_v1.components["schemas"]["Prediction"];
type PredictionList = OpenAPI.ai_prediction_v1.components["schemas"]["PredictionList"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

const STATUS_OPTIONS = ["pending", "approved", "rejected"];

export default async function AdminAiPredictionsPage({
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
      client.get<PredictionList>(`/api/v1/ai/predictions/predictions?${params}`),
      client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        "/api/v1/students?limit=100",
      ),
    ]);
    if (listResult.status === "fulfilled") {
      const rows = listResult.value.data ?? [];
      items = rows.map((row: Prediction) => ({
        id: row.id,
        student_id: row.student_id,
        itemType: row.prediction_type,
        title: row.title,
        detail: `score ${row.value.toLocaleString("en-GB", { maximumFractionDigits: 2 })}`,
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
          : "Failed to load predictions";
    }
    if (studentResult.status === "fulfilled") {
      const students: Student[] = studentResult.value.data ?? [];
      studentNames = new Map(
        students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
      );
    }
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load predictions";
  }

  const visible = query.status?.trim()
    ? items.filter((item) => item.status === query.status?.trim())
    : items;
  const pending = visible.filter((item) => item.status === "pending").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<TrendingUp className="size-7" />}
        title="AI predictions"
        description="Model predictions across the school — including unapproved ones staff have not yet reviewed. Learners never see items in pending or rejected states."
      />
      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Visible items" value={visible.length} unit="records" />
        <StatCard label="Awaiting review" value={pending} tone={pending > 0 ? "warn" : "default"} />
        <StatCard
          label="Rejected"
          value={visible.filter((item) => item.status === "rejected").length}
        />
      </div>
      <AdminAiOversightList
        basePath="/admin/ai/predictions"
        noun="predictions"
        items={visible}
        studentNames={studentNames}
        statusOptions={STATUS_OPTIONS}
        query={query}
        nextCursor={nextCursor}
        error={error}
        emptyTitle="No predictions yet"
        emptyDescription="Predictions generated from the feature store will appear here with their confidence and review status."
      />
    </div>
  );
}
