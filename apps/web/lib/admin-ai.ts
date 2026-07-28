export interface AiOversightItem {
  id: string;
  student_id: string;
  itemType: string;
  title: string;
  detail: string | null;
  status: string;
  confidence: number | null;
  explanation: string | null;
  created_at: string;
}

export function aiStatusStyle(status: string) {
  switch (status) {
    case "approved":
      return "bg-emerald-50 text-emerald-800";
    case "pending":
      return "bg-amber-50 text-amber-800";
    case "rejected":
      return "bg-red-50 text-red-800";
    case "overridden":
      return "bg-blue-50 text-blue-800";
    default:
      return "bg-muted text-muted-foreground";
  }
}

export function formatConfidence(confidence: number | null) {
  if (confidence == null) return "—";
  return `${Math.round(confidence * 100)}%`;
}
