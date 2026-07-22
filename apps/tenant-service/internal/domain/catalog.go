package domain

import "github.com/auraedu/platform/flags"

// FeatureCatalogEntry is a stable feature key + the plan tier that unlocks it
// (spec §9, §16). This is the canonical catalog; adapters build per-tenant
// snapshots from it, and entitlement checks read the plan from it.
type FeatureCatalogEntry struct {
	Key          string
	PlanRequired string
}

// FeatureCatalog returns the canonical feature catalog.
// Adapters build per-tenant snapshots from it, and entitlement checks read the plan from it.
func FeatureCatalog() []FeatureCatalogEntry {
	generated := flags.KnownFeatures()
	catalog := make([]FeatureCatalogEntry, 0, len(generated))
	for _, feature := range generated {
		catalog = append(catalog, FeatureCatalogEntry{
			Key:          feature.Key,
			PlanRequired: feature.PlanRequired,
		})
	}
	return catalog
}

// FeaturePlan returns the plan tier required for a feature key, and whether the key is known.
func FeaturePlan(key string) (string, bool) {
	for _, e := range FeatureCatalog() {
		if e.Key == key {
			return e.PlanRequired, true
		}
	}
	return "", false
}

// PlanAllows reports whether a tenant on `tenantPlan` is entitled to a feature that
// requires `featurePlan` (spec §3.3: billing controls which features a tenant may enable).
func PlanAllows(tenantPlan, featurePlan string) bool {
	if featurePlan == "" || featurePlan == "core" {
		return true
	}
	planRank := map[string]int{
		"core": 0, "starter": 1, "growth": 2, "professional": 3, "ai_plus": 4, "enterprise": 5,
	}
	tr, ok1 := planRank[tenantPlan]
	fr, ok2 := planRank[featurePlan]
	if !ok1 || !ok2 {
		return false
	}
	return tr >= fr
}
