import { Compass } from "lucide-react";
import { PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminAiOversightList, type AiOversightQuery } from "@/components/admin-ai-oversight";
import type { AiOversightItem } from "@/lib/admin-ai";

type Guidance = OpenAPI.career_guidance_v1.components["schemas"]["Guidance"];
type GuidanceList = OpenAPI.career_guidance_v1.components["schemas"]["GuidanceList"];
type Student = OpenAPI.student_v1.components["schemas"]["Student"];

const STATUS_OPTIONS = ["pending", "approved", "rejected"];

export default async function AdminAiGuidancePage({
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
      client.get<GuidanceList>(`/api/v1/ai/career-guidance/guidance?${params}`),
      client.get<OpenAPI.student_v1.components["schemas"]["StudentList"]>(
        "/api/v1/students?limit=100",
      ),
    ]);
    if (listResult.status === "fulfilled") {
      const rows = listResult.value.data ?? [];
      items = rows.map((row: Guidance) => ({
        id: row.id,
        student_id: row.student_id,
        itemType: row.guidance_type,
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
          : "Failed to load career guidance";
    }
    if (studentResult.status === "fulfilled") {
      const students: Student[] = studentResult.value.data ?? [];
      studentNames = new Map(
        students.map((student) => [student.id, `${student.first_name} ${student.last_name}`]),
      );
    }
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load career guidance";
  }

  const visible = query.status?.trim()
    ? items.filter((item) => item.status === query.status?.trim())
    : items;
  const pending = visible.filter((item) => item.status === "pending").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Compass className="size-7" />}
        title="Career guidance"
        description="AI career guidance awaiting and past staff review — the full pipeline, including items learners cannot yet see."
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
        basePath="/admin/ai/guidance"
        noun="career guidance"
        items={visible}
        studentNames={studentNames}
        statusOptions={STATUS_OPTIONS}
        query={query}
        nextCursor={nextCursor}
        error={error}
        emptyTitle="No guidance yet"
        emptyDescription="Career guidance generated for learners will appear here with its confidence and review status."
      />
    </div>
  );
}
