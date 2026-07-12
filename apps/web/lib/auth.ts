import { cookies } from "next/headers";

export interface UserSession {
  sub: string;
  user_id?: string;
  tenant_id?: string;
  role: string;
  email?: string;
  name?: string;
  permissions?: string[];
  exp?: number;
}

export const ACCESS_TOKEN_COOKIE = "auraedu_access_token";
export const REFRESH_TOKEN_COOKIE = "auraedu_refresh_token";
export const USER_COOKIE = "auraedu_user";

export const SUPER_ADMIN_ROLES = new Set(["platform_super_admin"]);
export const ADMIN_ROLES = new Set(["school_admin", "platform_super_admin", "super_admin"]);
export const TEACHER_ROLES = new Set(["teacher", "school_admin", "super_admin"]);
export const PARENT_ROLES = new Set(["parent", "school_admin", "super_admin"]);
export const STUDENT_ROLES = new Set(["student", "school_admin", "super_admin"]);

export function decodeJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const body = parts[1];
    if (!body) return null;
    const base64 = body.replace(/-/g, "+").replace(/_/g, "/");
    const pad = 4 - (base64.length % 4);
    const padded = pad === 4 ? base64 : base64 + "=".repeat(pad);
    const payload = atob(padded);
    return JSON.parse(payload) as Record<string, unknown>;
  } catch {
    return null;
  }
}

export function isTokenExpired(token: string, nowSeconds = Math.floor(Date.now() / 1000)): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload) return true;
  const exp = typeof payload.exp === "number" ? payload.exp : 0;
  if (!exp) return false;
  return nowSeconds >= exp;
}

export function toUserSession(token: string): UserSession | null {
  const payload = decodeJwtPayload(token);
  if (!payload) return null;
  const role = typeof payload.role === "string" ? payload.role : "";
  if (!role) return null;

  const perms = payload.permissions;
  const permissions = Array.isArray(perms)
    ? perms.filter((p): p is string => typeof p === "string")
    : undefined;

  return {
    sub: typeof payload.sub === "string" ? payload.sub : "",
    user_id: typeof payload.user_id === "string" ? payload.user_id : undefined,
    tenant_id: typeof payload.tenant_id === "string" ? payload.tenant_id : undefined,
    role,
    permissions,
    exp: typeof payload.exp === "number" ? payload.exp : undefined,
  };
}

export async function getSession(): Promise<UserSession | null> {
  const jar = await cookies();
  const token = jar.get(ACCESS_TOKEN_COOKIE)?.value;
  if (!token) return null;
  const session = toUserSession(token);
  if (!session) return null;

  const userCookie = jar.get(USER_COOKIE)?.value;
  if (userCookie) {
    try {
      const user = JSON.parse(userCookie) as {
        id?: string;
        email?: string;
        name?: string;
        role?: string;
        tenant_id?: string;
      };
      if (user.email) session.email = user.email;
      if (user.name) session.name = user.name;
      if (user.role) session.role = user.role;
      if (user.tenant_id) session.tenant_id = user.tenant_id;
      if (user.id && !session.user_id) session.user_id = user.id;
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

export function isSuperAdmin(session: UserSession): boolean {
  return SUPER_ADMIN_ROLES.has(session.role);
}

export function isAdmin(session: UserSession): boolean {
  return ADMIN_ROLES.has(session.role);
}

export function isTeacher(session: UserSession): boolean {
  return TEACHER_ROLES.has(session.role);
}

export function isParent(session: UserSession): boolean {
  return PARENT_ROLES.has(session.role);
}

export function isStudent(session: UserSession): boolean {
  return STUDENT_ROLES.has(session.role);
}

export function roleHomePath(role: string): string {
  if (SUPER_ADMIN_ROLES.has(role)) return "/superadmin";
  if (ADMIN_ROLES.has(role)) return "/admin";
  if (TEACHER_ROLES.has(role)) return "/teacher";
  if (PARENT_ROLES.has(role)) return "/parent";
  if (STUDENT_ROLES.has(role)) return "/student";
  return "/login";
}
