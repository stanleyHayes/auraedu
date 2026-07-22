package domain

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFeatureCatalogMatchesContract(t *testing.T) {
	contractPath := filepath.Join("..", "..", "..", "..", "contracts", "features", "features.yaml")
	contents, err := os.ReadFile(contractPath) //nolint:gosec // repository-owned contract fixture
	if err != nil {
		t.Fatalf("read feature contract: %v", err)
	}

	var contract struct {
		Features []struct {
			Key          string `yaml:"key"`
			PlanRequired string `yaml:"plan_required"`
		} `yaml:"features"`
	}
	if err := yaml.Unmarshal(contents, &contract); err != nil {
		t.Fatalf("parse feature contract: %v", err)
	}

	want := make([]FeatureCatalogEntry, 0, len(contract.Features))
	for _, feature := range contract.Features {
		want = append(want, FeatureCatalogEntry{
			Key:          feature.Key,
			PlanRequired: feature.PlanRequired,
		})
	}

	if got := FeatureCatalog(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tenant feature catalog drifted from contracts/features/features.yaml\n got: %#v\nwant: %#v", got, want)
	}
}
