import { publicApiUrl, tenantHeaderName } from "@auraedu/config";
import type { TenantData, FeatureFlag } from "@auraedu/shared-types";

export interface Branding {
  logo_url?: string;
  brand: { primary: string; secondary?: string };
}

export { type TenantData, type FeatureFlag };

export const SWITCHER = [
  { code: "upshs", short: "UPSHS", swatch: "#7B1113" },
  { code: "aboom-ame-zion-c", short: "Aboom", swatch: "#1E7D52" },
  { code: "cape-coast-prep", short: "Cape Coast", swatch: "#2456A6" },
] as const;

export const DEFAULT_CODE = SWITCHER[0].code;
export const DEFAULT_BRAND = SWITCHER[0].swatch;

export const PREVIEW_TENANT_CODES: readonly string[] = SWITCHER.map((school) => school.code);

export interface NavItemDef {
  label: string;
  href: string;
  feature?: string;
  badge?: number;
}
export interface NavGroupDef {
  heading: string;
  items: NavItemDef[];
}

export const NAV: NavGroupDef[] = [
  {
    heading: "People",
    items: [
      { label: "Students", href: "/students", feature: "student_management" },
      { label: "Staff", href: "/staff", feature: "staff_management" },
    ],
  },
  {
    heading: "Teaching",
    items: [
      { label: "Attendance", href: "/attendance", feature: "attendance" },
      { label: "Assignments", href: "/assignments", feature: "assignments" },
      { label: "Assessments", href: "/assessments", feature: "assessments" },
      { label: "Report cards", href: "/report-cards", feature: "report_cards" },
    ],
  },
  {
    heading: "Insight",
    items: [{ label: "Analytics", href: "/analytics", feature: "analytics" }],
  },
  {
    heading: "Money",
    items: [{ label: "Fees", href: "/fees", feature: "fees", badge: 3 }],
  },
];

export const TEACHER_NAV: NavGroupDef[] = [
  {
    heading: "Overview",
    items: [{ label: "Dashboard", href: "/teacher" }],
  },
  {
    heading: "Teaching",
    items: [
      { label: "My Classes", href: "/teacher/classes", feature: "academic_management" },
      { label: "Attendance", href: "/teacher/attendance", feature: "attendance" },
      { label: "Scores", href: "/teacher/scores", feature: "assessments" },
      { label: "Assignments", href: "/teacher/assignments", feature: "assignments" },
    ],
  },
  {
    heading: "Insight",
    items: [
      { label: "Reports", href: "/teacher/reports", feature: "report_cards" },
      { label: "Analytics", href: "/teacher/analytics", feature: "analytics" },
    ],
  },
];

export const SUPERADMIN_NAV: NavGroupDef[] = [
  {
    heading: "Overview",
    items: [{ label: "Dashboard", href: "/superadmin" }],
  },
  {
    heading: "Platform",
    items: [
      { label: "Tenants", href: "/superadmin/tenants" },
      { label: "System Health", href: "/superadmin/system-health" },
    ],
  },
  {
    heading: "Billing",
    items: [
      { label: "Billing Plans", href: "/superadmin/billing-plans" },
      { label: "Subscriptions", href: "/superadmin/subscriptions" },
    ],
  },
  {
    heading: "Compliance",
    items: [{ label: "Audit Logs", href: "/superadmin/audit-logs" }],
  },
];

export const ADMIN_NAV: NavGroupDef[] = [
  {
    heading: "Overview",
    items: [{ label: "Dashboard", href: "/admin" }],
  },
  {
    heading: "People",
    items: [
      { label: "Students", href: "/admin/students", feature: "student_management" },
      { label: "Staff", href: "/admin/staff", feature: "staff_management" },
    ],
  },
  {
    heading: "Academics",
    items: [
      { label: "Academic years", href: "/admin/academic-years", feature: "academic_management" },
      { label: "Classes", href: "/admin/classes", feature: "academic_management" },
      { label: "Subjects", href: "/admin/subjects", feature: "academic_management" },
    ],
  },
  {
    heading: "Operations",
    items: [
      { label: "Attendance", href: "/admin/attendance", feature: "attendance" },
      { label: "Assessments", href: "/admin/assessments", feature: "assessments" },
      { label: "Reports", href: "/admin/reports", feature: "report_cards" },
    ],
  },
  {
    heading: "Finance",
    items: [
      { label: "Fees", href: "/admin/fees", feature: "fees" },
      { label: "Payments", href: "/admin/payments", feature: "online_payments" },
    ],
  },
  {
    heading: "School",
    items: [
      { label: "Communications", href: "/admin/communications", feature: "announcements" },
      { label: "Website", href: "/admin/website", feature: "public_website" },
      { label: "Settings", href: "/admin/settings", feature: "settings" },
    ],
  },
];

export const STUDENT_NAV: NavGroupDef[] = [
  {
    heading: "Overview",
    items: [{ label: "Dashboard", href: "/student" }],
  },
  {
    heading: "Academic",
    items: [
      { label: "Timetable", href: "/student/timetable", feature: "timetable" },
      { label: "Assignments", href: "/student/assignments", feature: "assignments" },
      { label: "Results", href: "/student/results", feature: "assessments" },
      { label: "Report Card", href: "/student/report-card", feature: "report_cards" },
      { label: "CBT Exams", href: "/student/cbt-exams", feature: "cbt_exams" },
    ],
  },
  {
    heading: "Growth",
    items: [
      { label: "Recommendations", href: "/student/recommendations", feature: "ai_recommendations" },
    ],
  },
];

export const PARENT_NAV: NavGroupDef[] = [
  {
    heading: "Overview",
    items: [{ label: "Dashboard", href: "/parent" }],
  },
  {
    heading: "Family",
    items: [{ label: "My Children", href: "/parent/children", feature: "student_management" }],
  },
  {
    heading: "Academic",
    items: [
      { label: "Attendance", href: "/parent/attendance", feature: "attendance" },
      { label: "Results", href: "/parent/results", feature: "assessments" },
      { label: "Report Cards", href: "/parent/reports", feature: "report_cards" },
    ],
  },
  {
    heading: "Finance",
    items: [
      { label: "Fees", href: "/parent/fees", feature: "fees" },
      { label: "Payments", href: "/parent/payments", feature: "online_payments" },
    ],
  },
  {
    heading: "School",
    items: [
      { label: "Notifications", href: "/parent/notifications", feature: "announcements" },
      { label: "Guidance", href: "/parent/guidance", feature: "career_guidance" },
    ],
  },
];

export const TENANT_NOT_FOUND_HEADER = "x-tenant-not-found";

export function resolveTenantFromHost(host: string): string {
  const [name] = host.split(":");
  if (!name) return "";

  const lower = name.toLowerCase();
  if (lower === "localhost" || lower === "auraedu.com" || lower === "www.auraedu.com") {
    return DEFAULT_CODE;
  }

  if (lower.endsWith(".localhost") || lower.endsWith(".auraedu.com")) {
    const code = lower.split(".")[0] ?? "";
    return code.length > 0 ? code : "";
  }

  return "";
}

export function getTenantCodeFromHeaders(headers: Headers): string {
  const host = headers.get("host") ?? "";
  const headerCode = headers.get(tenantHeaderName);
  return headerCode ?? resolveTenantFromHost(host);
}

export function isTenantNotFound(headers: Headers): boolean {
  return headers.get(TENANT_NOT_FOUND_HEADER) === "1";
}

interface ResolvedTenant {
  tenant_code: string;
  name: string;
  short: string;
  status: string;
  plan: string;
  branding: Branding;
}

interface FeatureResponse {
  tenant_code: string;
  features: FeatureFlag[];
}

function tenantHeaders(code: string): HeadersInit {
  return {
    [tenantHeaderName]: code,
    "X-Tenant-ID": code,
  };
}

export async function fetchTenantBranding(code: string): Promise<TenantData> {
  if (!code) {
    throw new Error("tenant code is required");
  }

  const headers = tenantHeaders(code);
  const encoded = encodeURIComponent(code);

  const [resolveRes, featuresRes] = await Promise.all([
    fetch(`${publicApiUrl}/api/v1/tenants/resolve?subdomain=${encoded}`, {
      headers,
      next: { revalidate: 60 },
    }),
    fetch(`${publicApiUrl}/api/v1/features?tenant=${encoded}`, {
      headers,
      next: { revalidate: 60 },
    }),
  ]);

  if (!resolveRes.ok) {
    throw new Error(`tenant ${code} not found`);
  }

  const resolved = (await resolveRes.json()) as ResolvedTenant;
  const featureBody = featuresRes.ok ? ((await featuresRes.json()) as FeatureResponse) : { features: [] as FeatureFlag[] };

  return {
    code: resolved.tenant_code,
    name: resolved.name,
    short: resolved.short,
    plan: resolved.plan,
    branding: resolved.branding,
    features: featureBody.features,
  };
}

export function toFeatureSnapshot(tenant: TenantData): {
  tenantCode: string;
  flags: FeatureFlag[];
} {
  return { tenantCode: tenant.code, flags: tenant.features };
}
