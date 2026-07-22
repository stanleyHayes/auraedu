const roleHomes: Record<string, string> = {
  platform_super_admin: "/superadmin",
  school_admin: "/admin",
  super_admin: "/admin",
  teacher: "/teacher",
  parent: "/parent",
  student: "/student",
  applicant: "/applicant",
};

/** Returns a same-origin, role-owned post-login path or the role home. */
export function safePostLoginPath(candidate: string, role: string): string {
  const fallback = roleHomes[role] ?? "/login";
  const value = candidate.trim();
  if (
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\") ||
    value.length > 2048
  ) {
    return fallback;
  }
  let url: URL;
  try {
    url = new URL(value, "https://app.auraedu.invalid");
  } catch {
    return fallback;
  }
  if (url.origin !== "https://app.auraedu.invalid") return fallback;

  const allowedRoots =
    role === "platform_super_admin"
      ? ["/superadmin"]
      : role === "school_admin" || role === "super_admin"
        ? ["/admin", "/teacher", "/parent", "/student", "/applicant"]
        : roleHomes[role]
          ? [roleHomes[role]]
          : [];
  const allowed = allowedRoots.some(
    (root) => url.pathname === root || url.pathname.startsWith(`${root}/`),
  );
  return allowed ? `${url.pathname}${url.search}` : fallback;
}
