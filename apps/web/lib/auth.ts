import { cookies } from "next/headers";

export interface UserSession {
  sub: string;
  tenant_id?: string;
  role: string;
  email?: string;
  name?: string;
  perms?: string[];
  exp?: number;
}

const TOKEN_COOKIE = "auraedu_access_token";
const USER_COOKIE = "auraedu_user";

export const ADMIN_ROLES = new Set(["school_admin", "platform_super_admin", "super_admin"]);

export function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const body = parts[1];
    if (!body) return null;
    const payload = atob(body.replace(/-/g, "+").replace(/_/g, "/"));
    return JSON.parse(payload) as Record<string, unknown>;
  } catch {
    return null;
  }
}

export function toUserSession(token: string): UserSession | null {
  const payload = decodeJwtPayload(token);
  if (!payload) return null;
  const role = typeof payload.role === "string" ? payload.role : "";
  if (!role) return null;
  return {
    sub: typeof payload.sub === "string" ? payload.sub : "",
    tenant_id: typeof payload.tenant_id === "string" ? payload.tenant_id : undefined,
    role,
    email: typeof payload.email === "string" ? payload.email : undefined,
    name: typeof payload.name === "string" ? payload.name : undefined,
    perms: Array.isArray(payload.perms) ? payload.perms.filter((p): p is string => typeof p === "string") : undefined,
    exp: typeof payload.exp === "number" ? payload.exp : undefined,
  };
}

export async function getSession(): Promise<UserSession | null> {
  const jar = await cookies();
  const token = jar.get(TOKEN_COOKIE)?.value;
  if (!token) return null;
  const session = toUserSession(token);
  if (!session) return null;

  const userCookie = jar.get(USER_COOKIE)?.value;
  if (userCookie) {
    try {
      const user = JSON.parse(userCookie) as { email?: string; name?: string; role?: string };
      if (user.email) session.email = user.email;
      if (user.name) session.name = user.name;
      if (user.role) session.role = user.role;
    } catch {
      /* ignore */
    }
  }

  return session;
}

export async function requireAuth(): Promise<UserSession> {
  const session = await getSession();
  if (!session) {
    throw new Error("unauthenticated");
  }
  return session;
}

export function isAdmin(session: UserSession): boolean {
  return ADMIN_ROLES.has(session.role);
}
