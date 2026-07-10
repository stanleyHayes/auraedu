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

export interface ApiErrorEnvelope {
  code: string;
  message: string;
  details?: unknown;
  request_id?: string;
}

export interface LoginRequest {
  username: string;
  password: string;
  tenant_code?: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

export interface JwtClaims {
  tenant_id: string;
  user_id: string;
  role: string;
  permissions: string[];
  features_hash?: string;
}
