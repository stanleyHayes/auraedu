import { gatewayInternalUrl, tenantHeaderName } from "@auraedu/config";
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

export const APPLICANT_NAV: NavGroupDef[] = [
  {
    heading: "Admissions",
    items: [{ label: "My application", href: "/applicant", feature: "admissions" }],
  },
];

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
      { label: "Feature flags", href: "/superadmin/flags" },
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
    heading: "Growth",
    items: [
      { label: "Recruitment leads", href: "/admin/leads", feature: "growth_crm" },
      { label: "Communication journeys", href: "/admin/journeys", feature: "growth_crm" },
      { label: "Approved knowledge", href: "/admin/knowledge", feature: "growth_website_chat" },
      { label: "Campaign control", href: "/admin/campaigns", feature: "growth_campaigns" },
      { label: "Content studio", href: "/admin/content", feature: "growth_content_ai" },
      { label: "Programme catalogue", href: "/admin/programmes", feature: "admissions" },
      { label: "Admissions pipeline", href: "/admin/admissions", feature: "admissions" },
      { label: "Growth intelligence", href: "/admin/analytics", feature: "analytics" },
      {
        label: "Reputation desk",
        href: "/admin/intelligence?kind=reputation",
        feature: "growth_reputation_monitor",
      },
      {
        label: "Competitor watch",
        href: "/admin/intelligence?kind=competitor",
        feature: "growth_competitor_monitor",
      },
      {
        label: "AI action control",
        href: "/admin/automation",
        feature: "growth_autonomous_actions",
      },
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
      { label: "Settings", href: "/admin/settings" },
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
export const TENANT_CODE_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

/** Ink used for labels sitting on a light tenant brand. Mirrors --color-ink-950. */
const BRAND_CONTRAST_INK = "#061631";
const BRAND_CONTRAST_PAPER = "#ffffff";

function parseHex(color: string): [number, number, number] | null {
  const hex = color.trim().replace(/^#/, "");
  const full =
    hex.length === 3
      ? hex
          .split("")
          .map((c) => c + c)
          .join("")
      : hex;
  if (!/^[0-9a-f]{6}$/i.test(full)) return null;
  return [
    parseInt(full.slice(0, 2), 16),
    parseInt(full.slice(2, 4), 16),
    parseInt(full.slice(4, 6), 16),
  ];
}

/** WCAG relative luminance. */
function relativeLuminance([r, g, b]: [number, number, number]): number {
  const lin = [r, g, b].map((v) => {
    const c = v / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2];
}

/**
 * Pick the label colour for text sitting on a tenant's brand fill.
 *
 * Tenants choose arbitrary brand colours, so no fixed foreground works for all
 * of them: white is right on UPSHS maroon and wrong on a pale gold. Deriving it
 * from the brand's luminance keeps every tenant's primary button legible without
 * a per-school override, which rule 2 forbids.
 */
export function brandContrastColor(brand: string): string {
  const rgb = parseHex(brand);
  if (!rgb) return BRAND_CONTRAST_PAPER;
  const brandLum = relativeLuminance(rgb);
  const onPaper = 1.05 / (brandLum + 0.05);
  const onInk = (brandLum + 0.05) / (relativeLuminance([6, 22, 49]) + 0.05);
  return onPaper >= onInk ? BRAND_CONTRAST_PAPER : BRAND_CONTRAST_INK;
}

/** The dark-theme card surface these labels sit on. Mirrors --color-ink-900. */
const DARK_SURFACE: [number, number, number] = [11, 29, 58];
const TEXT_CONTRAST_TARGET = 4.5;

function toHex([r, g, b]: [number, number, number]): string {
  return "#" + [r, g, b].map((v) => Math.round(v).toString(16).padStart(2, "0")).join("");
}

/**
 * The brand used as *text* on a dark surface, rather than as a fill.
 *
 * A brand dark enough to need white button labels (UPSHS maroon) is far too dark
 * to read as text on the ink card — it lands near 1.5:1. Lift it toward white
 * only as far as legibility requires, so brands that already pass are untouched
 * and the tenant's hue is preserved as much as possible.
 */
export function brandOnDarkColor(brand: string): string {
  const rgb = parseHex(brand);
  if (!rgb) return BRAND_CONTRAST_PAPER;
  const surfaceLum = relativeLuminance(DARK_SURFACE);
  const contrastOf = (c: [number, number, number]) =>
    (relativeLuminance(c) + 0.05) / (surfaceLum + 0.05);

  if (contrastOf(rgb) >= TEXT_CONTRAST_TARGET) return toHex(rgb);

  for (let step = 1; step <= 20; step++) {
    const t = step / 20;
    const lifted: [number, number, number] = [
      rgb[0] + (255 - rgb[0]) * t,
      rgb[1] + (255 - rgb[1]) * t,
      rgb[2] + (255 - rgb[2]) * t,
    ];
    if (contrastOf(lifted) >= TEXT_CONTRAST_TARGET) return toHex(lifted);
  }
  return BRAND_CONTRAST_PAPER;
}

/**
 * Canonicalize user- or link-supplied workspace codes before they are allowed
 * to influence tenant headers or cookies. The value is deliberately bounded
 * to a single DNS label.
 */
export function canonicalTenantCode(value: string | null | undefined): string {
  const code = value?.trim().toLowerCase() ?? "";
  return TENANT_CODE_PATTERN.test(code) ? code : "";
}

export function isTenantNeutralAppHost(host: string): boolean {
  const [name] = host.split(":");
  return name?.toLowerCase() === "app.auraedu.com";
}

/**
 * Hosts without a tenant subdomain (localhost, the apex domain) carry no
 * tenant of their own, so the tenant they map to is deployment configuration,
 * never code. Unset means no tenant, which routes into the tenant-not-found
 * path.
 */
function defaultTenantCode(): string {
  return canonicalTenantCode(process.env.NEXT_PUBLIC_DEFAULT_TENANT_CODE);
}

export function resolveTenantFromHost(host: string): string {
  const [name] = host.split(":");
  if (!name) return "";

  const lower = name.toLowerCase();
  if (
    lower === "localhost" ||
    lower === "127.0.0.1" ||
    lower === "auraedu.com" ||
    lower === "www.auraedu.com"
  ) {
    return defaultTenantCode();
  }

  // The shared application hostname is a tenant-neutral authentication entry
  // point, not a school with the tenant code "app".
  if (isTenantNeutralAppHost(host)) return "";

  if (lower.endsWith(".localhost") || lower.endsWith(".auraedu.com")) {
    return canonicalTenantCode(lower.split(".")[0]);
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
    fetch(`${gatewayInternalUrl}/api/v1/tenants/resolve?subdomain=${encoded}`, {
      headers,
      next: { revalidate: 60 },
    }),
    fetch(`${gatewayInternalUrl}/api/v1/features?tenant=${encoded}`, {
      headers,
      next: { revalidate: 60 },
    }),
  ]);

  if (!resolveRes.ok) {
    throw new Error(`tenant ${code} not found`);
  }

  const resolved = (await resolveRes.json()) as ResolvedTenant;
  const featureBody = featuresRes.ok
    ? ((await featuresRes.json()) as FeatureResponse)
    : { features: [] as FeatureFlag[] };

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
