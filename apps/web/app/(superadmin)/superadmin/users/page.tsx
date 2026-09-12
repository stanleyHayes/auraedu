import { ShieldCheck, UsersRound } from "lucide-react";
import { DataTable, EmptyState, PageHeader, StatCard } from "@auraedu/ui";
import type { OpenAPI } from "@auraedu/shared-types";
import { createServerClient } from "@/lib/api";
import { requireAuth } from "@/lib/auth";
import { AdminUserInviteSheet } from "@/components/admin-user-invite-sheet";
import { AdminUserRoleActions } from "@/components/admin-user-role-actions";
import { AdminPermissionEditor } from "@/components/admin-permission-editor";

type User = OpenAPI.identity_v1.components["schemas"]["User"];
type UserList = OpenAPI.identity_v1.components["schemas"]["UserList"];
type RoleList = OpenAPI.identity_v1.components["schemas"]["RoleList"];
type PermissionList = OpenAPI.identity_v1.components["schemas"]["PermissionList"];

export default async function PlatformUsersPage() {
  const session = await requireAuth();
  const currentUserId = session.user_id ?? session.sub;

  let users: User[] = [];
  let roles: string[] = [];
  let permissions: string[] = [];
  let error: string | null = null;

  try {
    const client = await createServerClient();
    const [userResult, roleResult, permissionResult] = await Promise.all([
      client.get<UserList>("/api/v1/users"),
      client.get<RoleList>("/api/v1/roles"),
      client.get<PermissionList>("/api/v1/permissions"),
    ]);
    users = userResult.data ?? [];
    roles = (roleResult.data ?? []).map((entry) => entry.role);
    permissions = permissionResult.data ?? [];
  } catch (cause) {
    error = cause instanceof Error ? cause.message : "The identity directory is unavailable.";
  }

  const active = users.filter((user) => user.status === "active").length;
  const tenants = new Set(users.map((user) => user.tenant_id).filter(Boolean)).size;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ShieldCheck className="size-7" />}
        title="Users, roles & permissions"
        description="Govern platform and tenant identities from one audited directory. Role and permission changes remain constrained by your own grants."
        action={<AdminUserInviteSheet roles={roles} platformScope />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Identity accounts" value={users.length} unit="users" />
        <StatCard label="Active access" value={active} tone="ok" />
        <StatCard label="Tenant coverage" value={tenants} unit="schools" />
      </div>

      {error ? (
        <EmptyState
          title="Could not load the identity directory"
          description={error}
          icon={<UsersRound className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Platform identity directory"
          rows={users}
          keyExtractor={(user) => user.id}
          columns={[
            {
              key: "name",
              header: "Identity",
              cell: (user) => (
                <div>
                  <p className="font-semibold">{user.name}</p>
                  <p className="text-xs text-muted-foreground">{user.email}</p>
                </div>
              ),
            },
            {
              key: "tenant",
              header: "Scope",
              cell: (user) => (
                <span className="text-sm font-medium">{user.tenant_id || "Platform"}</span>
              ),
            },
            {
              key: "role",
              header: "Role",
              cell: (user) => (
                <span className="inline-flex rounded-full border border-border bg-muted px-2.5 py-1 text-xs font-semibold capitalize">
                  {user.role.replaceAll("_", " ")}
                </span>
              ),
            },
            {
              key: "status",
              header: "Status",
              cell: (user) => (
                <span
                  className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${
                    user.status === "active"
                      ? "bg-emerald-50 text-emerald-800"
                      : user.status === "locked"
                        ? "bg-red-50 text-red-800"
                        : "bg-muted text-muted-foreground"
                  }`}
                >
                  {user.status}
                </span>
              ),
            },
            {
              key: "actions",
              header: "Access controls",
              cell: (user) =>
                user.id === currentUserId ? (
                  <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-bold text-primary">
                    Current account
                  </span>
                ) : (
                  <div className="flex flex-col items-end gap-1.5">
                    <AdminUserRoleActions user={user} roles={roles} />
                    <AdminPermissionEditor user={user} catalogue={permissions} />
                  </div>
                ),
            },
          ]}
          empty={
            <EmptyState
              title="No identities found"
              description="Identity accounts appear here after platform or tenant provisioning."
              icon={<UsersRound className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
