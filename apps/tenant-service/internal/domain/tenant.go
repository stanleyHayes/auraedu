// Package domain holds the Tenant Service business rules (agent_plan §5, spec §3).
// A tenant is a school; every tenant has branding and a set of feature flags.
package domain

// Brand is a school's color palette. Primary replaces --color-brand in the UI
// (BRAND.md §5); the chalkboard neutrals stay constant across tenants.
type Brand struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary,omitempty"`
}

// Branding is a tenant's visual identity, served to web/mobile at startup.
type Branding struct {
	LogoURL string `json:"logo_url,omitempty"`
	Brand   Brand  `json:"brand"`
}

// Tenant is a school on the platform.
type Tenant struct {
	Code     string   `json:"tenant_code"`
	Name     string   `json:"name"`
	Short    string   `json:"short"`
	Status   string   `json:"status"` // active | onboarding | suspended
	Domain   string   `json:"domain,omitempty"`
	Plan     string   `json:"plan"`
	Branding Branding `json:"branding"`
}

// FeatureFlag is one independently switchable capability for a tenant (spec §3.2).
type FeatureFlag struct {
	Key          string `json:"feature_key"`
	Enabled      bool   `json:"is_enabled"`
	PlanRequired string `json:"plan_required,omitempty"`
}

// TenantUpdate is a partial update to a tenant. Nil fields mean "no change".
type TenantUpdate struct {
	Name     *string   `json:"name,omitempty"`
	Short    *string   `json:"short,omitempty"`
	Status   *string   `json:"status,omitempty"`
	Domain   *string   `json:"domain,omitempty"`
	Plan     *string   `json:"plan,omitempty"`
	Branding *Branding `json:"branding,omitempty"`
}

// Validate enforces tenant invariants.
func (t Tenant) Validate() error {
	if t.Code == "" {
		return ErrValidation
	}
	if t.Name == "" {
		return ErrValidation
	}
	return nil
}

// ApplyUpdate returns a copy of t with the non-nil fields of upd applied.
func (t Tenant) ApplyUpdate(upd TenantUpdate) Tenant {
	if upd.Name != nil {
		t.Name = *upd.Name
	}
	if upd.Short != nil {
		t.Short = *upd.Short
	}
	if upd.Status != nil {
		t.Status = *upd.Status
	}
	if upd.Domain != nil {
		t.Domain = *upd.Domain
	}
	if upd.Plan != nil {
		t.Plan = *upd.Plan
	}
	if upd.Branding != nil {
		t.Branding = *upd.Branding
	}
	return t
}

// ValidTenantStatuses returns the allowed tenant statuses.
func ValidTenantStatuses() []string { return []string{"active", "onboarding", "suspended"} }

// ValidPlans returns the allowed subscription plans.
func ValidPlans() []string {
	return []string{"core", "starter", "growth", "professional", "ai_plus", "enterprise"}
}

func inSlice(v string, ss []string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// ValidateUpdate checks the updated fields are legal.
func (upd TenantUpdate) ValidateUpdate() error {
	if upd.Status != nil && !inSlice(*upd.Status, ValidTenantStatuses()) {
		return ErrValidation
	}
	if upd.Plan != nil && !inSlice(*upd.Plan, ValidPlans()) {
		return ErrValidation
	}
	if upd.Name != nil && *upd.Name == "" {
		return ErrValidation
	}
	return nil
}
