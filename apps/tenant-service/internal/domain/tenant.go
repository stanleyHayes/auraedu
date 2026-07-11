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
