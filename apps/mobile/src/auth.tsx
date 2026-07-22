import AsyncStorage from "@react-native-async-storage/async-storage";
import Constants from "expo-constants";
import * as Notifications from "expo-notifications";
import * as SecureStore from "expo-secure-store";
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Platform } from "react-native";
import { createGatewayClient, type GatewayClient } from "@auraedu/api-client";
import {
  normalizeGatewayApiUrl,
  normalizeTenantBranding,
  normalizeTokenPair,
  parseStoredSession,
  type MobileSession,
  type TenantBranding,
  type TenantBrandingPayload,
  type TokenPairPayload,
} from "./auth-utils";

const sessionKey = "auraedu.session.v1";
const tenantKey = "auraedu.tenant.v1";
const deviceKey = "auraedu.device.v1";
const refreshLeewayMs = 30_000;

export type { MobileRole, TenantBranding } from "./auth-utils";
export type Session = MobileSession;
export type PushStatus = "checking" | "available" | "enabled" | "blocked" | "unavailable";

interface AuthState {
  ready: boolean;
  tenantCode: string;
  session: Session | null;
  features: Set<string>;
  featuresReady: boolean;
  branding: TenantBranding | null;
  client: GatewayClient | null;
  pushStatus: PushStatus;
  enablePushNotifications: () => Promise<PushStatus>;
  signIn: (input: { tenantCode: string; email: string; password: string }) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

function runtimeExtra() {
  return Constants.expoConfig?.extra as
    | {
        apiUrl?: unknown;
        appEnvironment?: unknown;
        easProjectId?: unknown;
      }
    | undefined;
}

export function gatewayApiUrl() {
  const extra = runtimeExtra();
  const publicOverride: unknown = process.env.EXPO_PUBLIC_API_URL;
  const configured =
    typeof publicOverride === "string"
      ? publicOverride
      : typeof extra?.apiUrl === "string"
        ? extra.apiUrl
        : "";
  return normalizeGatewayApiUrl(configured, extra?.appEnvironment !== "development");
}

function easProjectID() {
  const constants = Constants as unknown as {
    easConfig?: { projectId?: unknown } | null;
  };
  const projectID = constants.easConfig?.projectId;
  const extra = runtimeExtra();
  return typeof projectID === "string"
    ? projectID
    : typeof extra?.easProjectId === "string"
      ? extra.easProjectId
      : "";
}

async function responseBody(response: Response): Promise<TokenPairPayload> {
  try {
    return (await response.json()) as TokenPairPayload;
  } catch {
    return {};
  }
}

async function stableDeviceID() {
  const stored = await SecureStore.getItemAsync(deviceKey);
  if (stored) return stored;
  const created = `${Platform.OS}-${Date.now()}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
  await SecureStore.setItemAsync(deviceKey, created);
  return created;
}

async function registerPush(
  session: Session,
  request: typeof fetch,
  requestPermission: boolean,
): Promise<PushStatus> {
  if (Platform.OS !== "ios" && Platform.OS !== "android") return "unavailable";
  const projectId = easProjectID();
  if (!projectId) return "unavailable";
  if (Platform.OS === "android") {
    await Notifications.setNotificationChannelAsync("default", {
      name: "School updates",
      importance: Notifications.AndroidImportance.DEFAULT,
    });
  }
  let permission = await Notifications.getPermissionsAsync();
  if (
    permission.status !== Notifications.PermissionStatus.GRANTED &&
    requestPermission &&
    permission.canAskAgain
  ) {
    permission = await Notifications.requestPermissionsAsync();
  }
  if (permission.status !== Notifications.PermissionStatus.GRANTED) {
    return permission.canAskAgain ? "available" : "blocked";
  }
  const pushToken = (await Notifications.getExpoPushTokenAsync({ projectId })) as unknown as {
    data: string;
  };
  const deviceID = await stableDeviceID();
  const response = await request(`${gatewayApiUrl()}/api/v1/device-tokens`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
      "X-Tenant-Code": session.user.tenant_id,
    },
    body: JSON.stringify({ device_id: deviceID, platform: Platform.OS, token: pushToken.data }),
  });
  if (!response.ok) throw new Error("Push registration failed.");
  return "enabled";
}

async function resolveBranding(tenant: string): Promise<TenantBranding> {
  const response = await fetch(
    `${gatewayApiUrl()}/api/v1/tenants/resolve?subdomain=${encodeURIComponent(tenant)}`,
  );
  let body: TenantBrandingPayload = {};
  try {
    body = (await response.json()) as TenantBrandingPayload;
  } catch {
    // The generic message below deliberately hides an invalid upstream payload.
  }
  if (!response.ok) throw new Error(body.message ?? "School could not be resolved.");
  return normalizeTenantBranding(tenant, body);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [ready, setReady] = useState(false);
  const [tenantCode, setTenantCode] = useState("");
  const [session, setSession] = useState<Session | null>(null);
  const [features, setFeatures] = useState<Set<string>>(new Set());
  const [featuresReady, setFeaturesReady] = useState(false);
  const [branding, setBranding] = useState<TenantBranding | null>(null);
  const [pushStatus, setPushStatus] = useState<PushStatus>("checking");
  const sessionRef = useRef<Session | null>(null);
  const refreshRef = useRef<Promise<Session | null> | null>(null);

  const installSession = useCallback(async (next: Session) => {
    await SecureStore.setItemAsync(sessionKey, JSON.stringify(next));
    sessionRef.current = next;
    setSession(next);
  }, []);

  const clearSession = useCallback(async () => {
    await SecureStore.deleteItemAsync(sessionKey);
    sessionRef.current = null;
    setSession(null);
    setFeatures(new Set());
    setFeaturesReady(false);
    setBranding(null);
    setPushStatus("checking");
  }, []);

  useEffect(() => {
    void Promise.all([SecureStore.getItemAsync(sessionKey), AsyncStorage.getItem(tenantKey)])
      .then(async ([storedSession, storedTenant]) => {
        if (storedSession) {
          try {
            const restored = parseStoredSession(storedSession);
            sessionRef.current = restored;
            setSession(restored);
          } catch {
            await SecureStore.deleteItemAsync(sessionKey);
          }
        }
        if (storedTenant) setTenantCode(storedTenant);
      })
      .finally(() => setReady(true));
  }, []);

  const refreshSession = useCallback(async (): Promise<Session | null> => {
    if (refreshRef.current) return refreshRef.current;
    const current = sessionRef.current;
    if (!current) return null;

    const pending = (async () => {
      const response = await fetch(`${gatewayApiUrl()}/api/v1/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-Tenant-Code": current.user.tenant_id },
        body: JSON.stringify({ refresh_token: current.refreshToken }),
      });
      if (!response.ok) throw new Error("The session could not be renewed.");
      const next = normalizeTokenPair(await responseBody(response));
      if (next.user.id !== current.user.id || next.user.tenant_id !== current.user.tenant_id) {
        throw new Error("The renewed session identity did not match the active account.");
      }
      await installSession(next);
      return next;
    })()
      .catch(async () => {
        await clearSession();
        return null;
      })
      .finally(() => {
        refreshRef.current = null;
      });
    refreshRef.current = pending;
    return pending;
  }, [clearSession, installSession]);

  const authenticatedFetch = useCallback(
    async (input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> => {
      let active = sessionRef.current;
      if (active && Date.parse(active.expiresAt) <= Date.now() + refreshLeewayMs) {
        active = await refreshSession();
      }
      const headers = new Headers(init.headers);
      if (active) headers.set("Authorization", `Bearer ${active.accessToken}`);
      let response = await fetch(input, { ...init, headers });
      if (response.status !== 401 || !active) return response;

      active = await refreshSession();
      if (!active) return response;
      headers.set("Authorization", `Bearer ${active.accessToken}`);
      response = await fetch(input, { ...init, headers });
      return response;
    },
    [refreshSession],
  );

  const loadFeatures = useCallback(
    async (tenant: string) => {
      setFeaturesReady(false);
      try {
        const response = await authenticatedFetch(
          `${gatewayApiUrl()}/api/v1/features?tenant=${encodeURIComponent(tenant)}`,
          {
            headers: { "X-Tenant-Code": tenant },
          },
        );
        if (!response.ok) throw new Error("Feature snapshot unavailable.");
        const body = (await response.json()) as {
          features?: { feature_key: string; is_enabled: boolean }[];
        };
        setFeatures(
          new Set(
            (body.features ?? []).filter((flag) => flag.is_enabled).map((flag) => flag.feature_key),
          ),
        );
      } catch {
        setFeatures(new Set());
      }
      setFeaturesReady(true);
    },
    [authenticatedFetch],
  );

  const signIn = useCallback(
    async (input: { tenantCode: string; email: string; password: string }) => {
      const tenant = input.tenantCode.trim().toLowerCase();
      const resolvedBranding = await resolveBranding(tenant);
      const response = await fetch(`${gatewayApiUrl()}/api/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-Tenant-Code": tenant },
        body: JSON.stringify({ email: input.email.trim().toLowerCase(), password: input.password }),
      });
      const body = await responseBody(response);
      if (!response.ok) {
        throw new Error(typeof body.message === "string" ? body.message : "Sign-in failed.");
      }
      const next = normalizeTokenPair(body);
      if (next.user.tenant_id !== tenant)
        throw new Error("The account does not belong to this school.");
      await Promise.all([installSession(next), AsyncStorage.setItem(tenantKey, tenant)]);
      setTenantCode(tenant);
      setBranding(resolvedBranding);
      await loadFeatures(tenant);
    },
    [installSession, loadFeatures],
  );

  useEffect(() => {
    if (ready && session && !featuresReady) void loadFeatures(session.user.tenant_id);
  }, [featuresReady, loadFeatures, ready, session]);

  useEffect(() => {
    if (!ready || !session || branding?.code === session.user.tenant_id) return;
    void resolveBranding(session.user.tenant_id)
      .then(setBranding)
      .catch(() => setBranding(null));
  }, [branding?.code, ready, session]);

  useEffect(() => {
    if (!ready || !session) return;
    void registerPush(session, authenticatedFetch, false)
      .then(setPushStatus)
      .catch(() => setPushStatus("unavailable"));
  }, [authenticatedFetch, ready, session]);

  const enablePushNotifications = useCallback(async (): Promise<PushStatus> => {
    const active = sessionRef.current;
    if (!active) return "unavailable";
    const next = await registerPush(active, authenticatedFetch, true);
    setPushStatus(next);
    return next;
  }, [authenticatedFetch]);

  const signOut = useCallback(async () => {
    let active = sessionRef.current;
    try {
      if (active && Date.parse(active.expiresAt) <= Date.now() + refreshLeewayMs) {
        active = await refreshSession();
      }
      if (active) {
        const deviceID = await SecureStore.getItemAsync(deviceKey);
        if (deviceID) {
          await fetch(`${gatewayApiUrl()}/api/v1/device-tokens/${encodeURIComponent(deviceID)}`, {
            method: "DELETE",
            headers: {
              Authorization: `Bearer ${active.accessToken}`,
              "X-Tenant-Code": active.user.tenant_id,
            },
          }).catch(() => undefined);
        }
        await fetch(`${gatewayApiUrl()}/api/v1/auth/logout`, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${active.accessToken}`,
            "Content-Type": "application/json",
            "X-Tenant-Code": active.user.tenant_id,
          },
          body: JSON.stringify({ refresh_token: active.refreshToken }),
        }).catch(() => undefined);
      }
    } finally {
      await clearSession();
    }
  }, [clearSession, refreshSession]);

  const client = useMemo(
    () =>
      createGatewayClient({
        baseUrl: gatewayApiUrl(),
        getToken: () => sessionRef.current?.accessToken,
        getTenantCode: () => sessionRef.current?.user.tenant_id,
        fetch: authenticatedFetch,
      }),
    [authenticatedFetch],
  );
  const availableClient = session ? client : null;
  const value = useMemo(
    () => ({
      ready,
      tenantCode,
      session,
      features,
      featuresReady,
      branding,
      client: availableClient,
      pushStatus,
      enablePushNotifications,
      signIn,
      signOut,
    }),
    [
      availableClient,
      branding,
      enablePushNotifications,
      features,
      featuresReady,
      pushStatus,
      ready,
      session,
      signIn,
      signOut,
      tenantCode,
    ],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
