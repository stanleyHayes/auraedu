package auth

import "testing"

func TestGeneratedAuthorizationRegistryIsCompleteAndImmutable(t *testing.T) {
	for _, permission := range []string{"users.update", "files.update", "analytics.executive.read", "intelligence.review"} {
		if !IsKnownPermission(permission) {
			t.Fatalf("missing permission %q", permission)
		}
	}
	for role, wantScope := range map[string]string{
		RolePlatformSuperAdmin: "all_tenants",
		"applicant":            "own_applications",
		"support_agent":        "limited_platform_support",
	} {
		if scope, ok := RoleScope(role); !ok || scope != wantScope {
			t.Fatalf("role %q scope=%q known=%v, want %q", role, scope, ok, wantScope)
		}
	}
	if IsKnownPermission("root.everything") || IsKnownRole("wizard") {
		t.Fatal("unknown authorization keys must fail closed")
	}

	permissions := KnownPermissions()
	permissions[0] = "mutated"
	if IsKnownPermission("mutated") || !IsKnownPermission("academic.manage") {
		t.Fatal("callers must not mutate the permission registry")
	}
	roles := KnownRoles()
	roles[0].Name = "mutated"
	if IsKnownRole("mutated") || !IsKnownRole(RolePlatformSuperAdmin) {
		t.Fatal("callers must not mutate the role registry")
	}
}
