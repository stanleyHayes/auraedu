import type { FeatureFlag } from "@auraedu/shared-types";

export function isFeatureEnabled(flags: FeatureFlag[], key: string): boolean {
  return flags.some((f) => f.feature_key === key && f.is_enabled);
}

interface RouteMapping {
  prefix: string;
  feature: string | null;
}

const ROUTE_FEATURES: RouteMapping[] = [
  // Admin
  { prefix: "/admin/students", feature: "student_management" },
  { prefix: "/admin/staff", feature: "staff_management" },
  { prefix: "/admin/academic-years", feature: "academic_management" },
  { prefix: "/admin/classes", feature: "academic_management" },
  { prefix: "/admin/subjects", feature: "academic_management" },
  { prefix: "/admin/attendance", feature: "attendance" },
  { prefix: "/admin/assessments", feature: "assessments" },
  { prefix: "/admin/reports", feature: "report_cards" },
  { prefix: "/admin/fees", feature: "fees" },
  { prefix: "/admin/payments", feature: "online_payments" },
  { prefix: "/admin/communications", feature: "announcements" },
  { prefix: "/admin/website", feature: "public_website" },
  { prefix: "/admin/leads", feature: "growth_crm" },
  { prefix: "/admin/journeys", feature: "growth_crm" },
  { prefix: "/admin/knowledge", feature: "growth_website_chat" },
  { prefix: "/admin/campaigns", feature: "growth_campaigns" },
  { prefix: "/admin/content", feature: "growth_content_ai" },
  { prefix: "/admin/admissions", feature: "admissions" },
  { prefix: "/admin/programmes", feature: "admissions" },
  { prefix: "/admin/analytics", feature: "analytics" },
  { prefix: "/admin/automation", feature: "growth_autonomous_actions" },
  { prefix: "/applicant", feature: "admissions" },

  // Teacher
  { prefix: "/teacher/classes", feature: "academic_management" },
  { prefix: "/teacher/attendance", feature: "attendance" },
  { prefix: "/teacher/scores", feature: "assessments" },
  { prefix: "/teacher/assignments", feature: "assignments" },
  { prefix: "/teacher/reports", feature: "report_cards" },
  { prefix: "/teacher/analytics", feature: "analytics" },

  // Student
  { prefix: "/student/timetable", feature: "timetable" },
  { prefix: "/student/assignments", feature: "assignments" },
  { prefix: "/student/results", feature: "assessments" },
  { prefix: "/student/report-card", feature: "report_cards" },
  { prefix: "/student/cbt-exams", feature: "cbt_exams" },
  { prefix: "/student/recommendations", feature: "ai_recommendations" },

  // Parent
  { prefix: "/parent/children", feature: "student_management" },
  { prefix: "/parent/attendance", feature: "attendance" },
  { prefix: "/parent/results", feature: "assessments" },
  { prefix: "/parent/reports", feature: "report_cards" },
  { prefix: "/parent/fees", feature: "fees" },
  { prefix: "/parent/payments", feature: "online_payments" },
  { prefix: "/parent/notifications", feature: "announcements" },
  { prefix: "/parent/guidance", feature: "career_guidance" },
];

export function getRouteFeature(pathname: string): string | null {
  const normalized = pathname.endsWith("/") && pathname !== "/" ? pathname.slice(0, -1) : pathname;
  for (const route of ROUTE_FEATURES) {
    if (normalized === route.prefix || normalized.startsWith(`${route.prefix}/`)) {
      return route.feature;
    }
  }
  return null;
}

export function enabledFeatureKeys(flags: FeatureFlag[] | null | undefined): Set<string> {
  return new Set((flags ?? []).filter((flag) => flag.is_enabled).map((flag) => flag.feature_key));
}

export function isNavigationFeatureVisible(
  feature: string | undefined,
  enabled: Set<string> | null,
  allowStub = false,
): boolean {
  if (!feature) return true;
  if (allowStub) return true;
  return enabled?.has(feature) === true;
}

export function checkRouteFeature(
  pathname: string,
  flags: FeatureFlag[],
): { enabled: boolean; feature: string | null } {
  const feature = getRouteFeature(pathname);
  if (!feature) return { enabled: true, feature: null };
  return { enabled: isFeatureEnabled(flags, feature), feature };
}
