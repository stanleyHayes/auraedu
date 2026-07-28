import { UsersRound } from "lucide-react";
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

export default async function AdminUsersPage() {
  await requireAuth();

  let users: User[] = [];
  let roles: string[] = [];
  let permissionCatalogue: string[] = [];
  let error: string | null = null;
  let rolesError: string | null = null;
  let permissionsError: string | null = null;

  try {
    const client = await createServerClient();
    const [userResult, roleResult, permissionResult] = await Promise.allSettled([
      client.get<UserList>("/api/v1/users"),
      client.get<RoleList>("/api/v1/roles"),
      client.get<PermissionList>("/api/v1/permissions"),
    ]);
    if (userResult.status === "fulfilled") {
      users = userResult.value.data ?? [];
    } else {
      error =
        userResult.reason instanceof Error
          ? userResult.reason.message
          : "Failed to load identity users";
    }
    if (roleResult.status === "fulfilled") {
      roles = (roleResult.value.data ?? [])
        .filter((entry) => entry.scope !== "all_tenants")
        .map((entry) => entry.role);
    } else {
      rolesError =
        roleResult.reason instanceof Error
          ? roleResult.reason.message
          : "Failed to load assignable roles";
    }
    if (permissionResult.status === "fulfilled") {
      permissionCatalogue = permissionResult.value.data ?? [];
    } else {
      permissionsError =
        permissionResult.reason instanceof Error
          ? permissionResult.reason.message
          : "Failed to load the permission catalogue";
    }
  } catch (e) {
    error = e instanceof Error ? e.message : "Failed to load identity users";
  }

  const active = users.filter((user) => user.status === "active").length;
  const locked = users.filter((user) => user.status === "locked").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<UsersRound className="size-7" />}
        title="Users & roles"
        description="Invite staff and families, keep roles honest, and deactivate access the moment it is no longer needed."
        action={<AdminUserInviteSheet roles={roles} />}
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Identity accounts" value={users.length} unit="users" />
        <StatCard label="Active" value={active} tone="ok" />
        <StatCard label="Locked" value={locked} tone={locked > 0 ? "warn" : "default"} />
      </div>

      {rolesError ? (
        <div className="rounded-xl border border-amber-300/60 bg-amber-50 p-4 text-sm text-amber-950">
          Assignable roles are unavailable: {rolesError}. Invitations and role changes are paused
          until the role list loads; existing accounts remain visible below.
        </div>
      ) : null}

      {permissionsError ? (
        <div className="rounded-xl border border-amber-300/60 bg-amber-50 p-4 text-sm text-amber-950">
          The permission catalogue is unavailable: {permissionsError}. The permission editor is
          paused until it loads; accounts and roles remain manageable below.
        </div>
      ) : null}

      {error ? (
        <EmptyState
          title="Could not load users"
          description={error}
          icon={<UsersRound className="size-8" />}
        />
      ) : (
        <DataTable
          caption="Identity users"
          rows={users}
          keyExtractor={(user) => user.id}
          columns={[
            {
              key: "name",
              header: "Name",
              cell: (user) => <span className="font-semibold">{user.name}</span>,
            },
            {
              key: "email",
              header: "Email",
              cell: (user) => <span className="text-sm">{user.email}</span>,
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
              cell: (user) => {
                const style =
                  user.status === "active"
                    ? "bg-emerald-50 text-emerald-800"
                    : user.status === "locked"
                      ? "bg-red-50 text-red-800"
                      : "bg-muted text-muted-foreground";
                return (
                  <span
                    className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${style}`}
                  >
                    {user.status}
                  </span>
                );
              },
            },
            {
              key: "actions",
              header: "Actions",
              cell: (user) => (
                <div className="flex flex-col items-end gap-1.5">
                  <AdminUserRoleActions user={user} roles={roles} />
                  <AdminPermissionEditor user={user} catalogue={permissionCatalogue} />
                </div>
              ),
            },
          ]}
          empty={
            <EmptyState
              title="No identity users yet"
              description="Send the first invitation above; accounts appear here once the identity service provisions them."
              icon={<UsersRound className="size-8" />}
            />
          }
        />
      )}
    </div>
  );
}
