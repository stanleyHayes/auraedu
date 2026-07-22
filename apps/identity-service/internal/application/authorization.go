package application

import (
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/platform/auth"
)

func validateAuthorizationGrant(actor auth.Actor, tenantID, role string, permissions []string) error {
	scope, known := auth.RoleScope(role)
	if !known {
		return domain.ErrValidation
	}
	platformRole := scope == "all_tenants" || scope == "limited_platform_support"
	if platformRole {
		if !actor.PlatformAdmin {
			return domain.ErrForbidden
		}
		if tenantID != "" {
			return domain.ErrValidation
		}
	} else if tenantID == "" {
		return domain.ErrValidation
	}

	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if !auth.IsKnownPermission(permission) {
			return domain.ErrValidation
		}
		if _, duplicate := seen[permission]; duplicate {
			return domain.ErrValidation
		}
		seen[permission] = struct{}{}
		if !actor.PlatformAdmin && !actor.Has(permission) {
			return domain.ErrForbidden
		}
	}
	return nil
}

func validUserStatus(status domain.UserStatus) bool {
	switch status {
	case domain.StatusActive, domain.StatusInactive, domain.StatusLocked:
		return true
	default:
		return false
	}
}
