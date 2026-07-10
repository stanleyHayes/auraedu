export interface FeatureFlag {
  feature_key: string;
  is_enabled: boolean;
  plan_required?: string;
}

export interface Branding {
  logo_url?: string;
  brand: { primary: string; secondary?: string };
}

export interface TenantData {
  code: string;
  name: string;
  short: string;
  plan: string;
  branding: Branding;
  features: FeatureFlag[];
}

/**
 * Schools for the "Preview as" switcher — a dev affordance until real tenant
 * resolution by subdomain lands via the gateway (EP-05, AURA-5.4). Branding and
 * feature flags are fetched live from the Tenant Service; only these codes/labels
 * are local, so the switcher can list schools without a super-admin call.
 */
export const SWITCHER = [
  { code: "upshs", short: "UPSHS", swatch: "#7B1113" },
  { code: "aboom-ame-zion-c", short: "Aboom", swatch: "#1E7D52" },
  { code: "cape-coast-prep", short: "Cape Coast", swatch: "#2456A6" },
] as const;

export const DEFAULT_CODE = SWITCHER[0].code;
export const DEFAULT_BRAND = SWITCHER[0].swatch;

/**
 * Tenant codes the demo preview proxy (`/api/tenant/[code]`) is allowed to serve.
 * A guardrail for the temporary forged-actor shim: it may only read these known
 * demo tenants, never arbitrary ones. Retired when the gateway injects the real actor.
 */
export const PREVIEW_TENANT_CODES: readonly string[] = SWITCHER.map((school) => school.code);

/** Nav item tagged with the feature flag that gates it (undefined = always shown). */
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

/** Portal nav — items appear only when their feature is enabled for the tenant. */
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

/**
 * Fetch a tenant's record + feature snapshot from the Tenant Service, via the app's
 * server-side proxy route (`/api/tenant/[code]`). Real resolution is by subdomain
 * through the gateway; this powers the preview switcher today.
 */
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
