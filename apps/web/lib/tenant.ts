export interface Tenant {
  code: string;
  name: string;
  short: string;
  /** School brand colour — replaces --color-brand at runtime (BRAND.md §5). */
  brand: string;
}

export const TENANTS: Tenant[] = [
  { code: "upshs", name: "University Practice SHS", short: "UPSHS", brand: "#7B1113" },
  { code: "aboom-ame-zion-c", name: "Aboom AME Zion C Basic", short: "Aboom", brand: "#1E7D52" },
  { code: "cape-coast-prep", name: "Cape Coast Prep", short: "Cape Coast", brand: "#2456A6" },
];

export const DEFAULT_TENANT: Tenant = TENANTS[0]!;

/**
 * STUB — real resolution is by host/subdomain via the Tenant Service (EP-05, AURA-5.4).
 * Returns the default tenant for now; the school switcher in the topbar previews others.
 */
export function resolveTenant(): Tenant {
  return DEFAULT_TENANT;
}
