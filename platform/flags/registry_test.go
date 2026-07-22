package flags

import "testing"

func TestGeneratedFeatureRegistry(t *testing.T) {
	features := KnownFeatures()
	if len(features) == 0 {
		t.Fatal("generated feature registry is empty")
	}

	seen := make(map[string]struct{}, len(features))
	for _, feature := range features {
		if feature.Key == "" || feature.PlanRequired == "" {
			t.Fatalf("incomplete generated feature: %#v", feature)
		}
		if _, exists := seen[feature.Key]; exists {
			t.Fatalf("duplicate generated feature %q", feature.Key)
		}
		seen[feature.Key] = struct{}{}
		if !IsKnownFeature(feature.Key) {
			t.Fatalf("generated feature %q missing from lookup set", feature.Key)
		}
	}

	for _, key := range []string{FeatureAdmissions, FeaturePushNotifications, FeatureGrowthCrm, FeatureGrowthWebsiteChat} {
		if !IsKnownFeature(key) {
			t.Fatalf("expected generated feature %q", key)
		}
	}
	if IsKnownFeature("unknown_feature") {
		t.Fatal("unknown feature must fail closed")
	}

	features[0].Key = "mutated"
	if KnownFeatures()[0].Key == "mutated" {
		t.Fatal("KnownFeatures returned mutable registry storage")
	}
}
