// Generated barrel — safe to re-export generated modules.
export * as OpenAPI from "./generated/openapi/index.js";
export * as Events from "./generated/events/index.js";

// Tenant / auth shapes consumed by the web shell until they are generated from contracts.
export interface FeatureFlag {
  feature_key: string;
  is_enabled: boolean;
  plan_required?: string;
}

export interface TenantBranding {
  logo_url?: string;
  brand: { primary: string; secondary?: string };
  display_font?: string;
}

export interface TenantData {
  code: string;
  name: string;
  short: string;
  plan: string;
  branding: TenantBranding;
  features: FeatureFlag[];
}
