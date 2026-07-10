export interface FeatureFlag {
  feature_key: string;
  is_enabled: boolean;
  plan_required?: string;
}

export interface GatewayClientOptions {
  baseUrl: string;
  tenantHeader?: string;
  getToken?: () => string | null | undefined;
  getTenantCode?: () => string | null | undefined;
  fetch?: typeof fetch;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown;
}

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export class FeatureDisabledError extends ApiError {
  constructor(public readonly feature: string, message: string) {
    super(403, "feature_disabled", message);
    this.name = "FeatureDisabledError";
  }
}

export class UnauthorizedError extends ApiError {
  constructor(message = "Unauthorized") {
    super(401, "unauthorized", message);
    this.name = "UnauthorizedError";
  }
}

function normalizeBase(url: string): string {
  return url.replace(/\/$/, "");
}

async function parseErrorBody(response: Response): Promise<ApiErrorBody> {
  try {
    const json = (await response.json()) as Record<string, unknown>;
    return {
      code: typeof json.code === "string" ? json.code : "unknown_error",
      message: typeof json.message === "string" ? json.message : response.statusText,
      details: json.details,
    };
  } catch {
    return { code: "unknown_error", message: response.statusText };
  }
}

export interface GatewayClient {
  get<T>(path: string, init?: RequestInit): Promise<T>;
  post<T>(path: string, body: unknown, init?: RequestInit): Promise<T>;
  patch<T>(path: string, body: unknown, init?: RequestInit): Promise<T>;
  put<T>(path: string, body: unknown, init?: RequestInit): Promise<T>;
  del<T>(path: string, init?: RequestInit): Promise<T>;
  request<T>(method: string, path: string, body?: unknown, init?: RequestInit): Promise<T>;
}

export function createGatewayClient(options: GatewayClientOptions): GatewayClient {
  const baseUrl = normalizeBase(options.baseUrl);
  const tenantHeader = options.tenantHeader ?? "x-tenant-code";
  const doFetch = options.fetch ?? fetch;

  async function request<T>(method: string, path: string, body?: unknown, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    headers.set("accept", "application/json");
    if (body !== undefined && !headers.has("content-type")) headers.set("content-type", "application/json");

    const token = options.getToken?.();
    if (token) headers.set("authorization", `Bearer ${token}`);

    const tenantCode = options.getTenantCode?.();
    if (tenantCode) headers.set(tenantHeader, tenantCode);

    const url = `${baseUrl}${path.startsWith("/") ? path : `/${path}`}`;
    const response = await doFetch(url, {
      ...init,
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });

    if (response.status === 204) return undefined as T;

    if (!response.ok) {
      const error = await parseErrorBody(response);
      if (response.status === 403 && error.code === "feature_disabled") {
        const feature = typeof error.details === "string" ? error.details : "unknown";
        throw new FeatureDisabledError(feature, error.message);
      }
      if (response.status === 401) throw new UnauthorizedError(error.message);
      throw new ApiError(response.status, error.code, error.message, error.details);
    }

    if (response.headers.get("content-length") === "0") return undefined as T;
    return (await response.json()) as T;
  }

  return {
    request,
    get: <T>(path: string, init?: RequestInit) => request<T>("GET", path, undefined, init),
    post: <T>(path: string, body: unknown, init?: RequestInit) => request<T>("POST", path, body, init),
    patch: <T>(path: string, body: unknown, init?: RequestInit) => request<T>("PATCH", path, body, init),
    put: <T>(path: string, body: unknown, init?: RequestInit) => request<T>("PUT", path, body, init),
    del: <T>(path: string, init?: RequestInit) => request<T>("DELETE", path, undefined, init),
  };
}

export function toFeatureSnapshot(tenantCode: string, flags: FeatureFlag[]): { tenantCode: string; flags: FeatureFlag[] } {
  return { tenantCode, flags };
}
