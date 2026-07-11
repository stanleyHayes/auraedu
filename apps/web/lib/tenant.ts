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
      { label: "Payments", href: "/admin/payments", feature: "payments" },
    ],
  },
  {
    heading: "School",
    items: [
      { label: "Communications", href: "/admin/communications", feature: "communications" },
      { label: "Website", href: "/admin/website", feature: "website" },
      { label: "Settings", href: "/admin/settings", feature: "settings" },
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
      { label: "Payments", href: "/parent/payments", feature: "payments" },
    ],
  },
  {
    heading: "School",
    items: [
      { label: "Notifications", href: "/parent/notifications", feature: "communications" },
      { label: "Guidance", href: "/parent/guidance", feature: "guidance" },
    ],
  },
];

export function resolveTenantFromHost(host: string): string {
  const [name] = host.split(":");
  if (!name) return DEFAULT_CODE;

  const lower = name.toLowerCase();
  if (lower === "localhost" || lower === "auraedu.com" || lower === "www.auraedu.com") {
    return DEFAULT_CODE;
  }

  if (lower.endsWith(".localhost") || lower.endsWith(".auraedu.com")) {
    const code = lower.split(".")[0];
    return code || DEFAULT_CODE;
  }

  return DEFAULT_CODE;
}

export function getTenantCodeFromHeaders(headers: Headers): string {
  const host = headers.get("host") ?? "";
  const headerCode = headers.get(tenantHeaderName);
  return headerCode ?? resolveTenantFromHost(host);
}

export async function fetchTenant(code: string): Promise<TenantData> {
  const res = await fetch(`/api/tenant/${code}`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`tenant ${code} failed to load`);
  }
  const json = (await res.json()) as {
    tenant: { tenant_code: string; name: string; short: string; plan: string; branding: Branding };
    features: FeatureFlag[];
  };
  return {
    code: json.tenant.tenant_code,
    name: json.tenant.name,
    short: json.tenant.short,
    plan: json.tenant.plan,
    branding: json.tenant.branding,
    features: json.features,
  };
}

export async function fetchTenantBranding(code: string): Promise<TenantData> {
  try {
    const res = await fetch(`${publicApiUrl}/api/v1/tenant/branding?tenant=${encodeURIComponent(code)}`, {
      headers: { [tenantHeaderName]: code },
      next: { revalidate: 60 },
    });
    if (!res.ok) throw new Error(`branding fetch failed: ${res.status}`);
    const data = (await res.json()) as TenantData;
    return data;
  } catch {
    return makeFallbackTenant(code);
  }
}

export function makeFallbackTenant(code: string): TenantData {
  const known = SWITCHER.find((s) => s.code === code);
  return {
    code,
    name: known?.short ?? code,
    short: known?.short ?? code,
    plan: "starter",
    branding: {
      brand: { primary: known?.swatch ?? DEFAULT_BRAND },
    },
    features: [],
  };
}

export function toFeatureSnapshot(tenant: TenantData): { tenantCode: string; flags: FeatureFlag[] } {
  return { tenantCode: tenant.code, flags: tenant.features };
}
