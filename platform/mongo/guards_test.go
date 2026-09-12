package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestTenantUpdateBypassRegression(t *testing.T) {
	for name, update := range map[string]bson.M{
		"rename destination":        {"$rename": bson.M{"other": "tenant_id"}},
		"rename dotted destination": {"$rename": bson.M{"other": "tenant_id.child"}},
		"dotted source":             {"$set": bson.M{"tenant_id.child": "other"}},
		"ordered document":          {"$unset": bson.D{{Key: "tenant_id", Value: 1}}},
		"ordinary map":              {"$set": map[string]string{"tenant_id": "other"}},
		"increment":                 {"$inc": bson.M{"tenant_id": 1}},
		"replacement":               {"name": "replacement"},
	} {
		t.Run(name, func(t *testing.T) {
			if guardUpdate(update, "tenant_id") == nil {
				t.Fatal("unsafe update accepted")
			}
		})
	}
	if err := guardUpdate(bson.M{"$set": bson.D{{Key: "name", Value: "okay"}}}, "tenant_id"); err != nil {
		t.Fatal(err)
	}
	if err := guardUpdate(bson.M{"$unset": bson.M{"owner": 1}}, "owner.tenant_id"); err == nil {
		t.Fatal("parent update accepted")
	}
}

func TestAggregationCrossCollectionRegression(t *testing.T) {
	raw, err := bson.Marshal(bson.M{"$lookup": bson.M{"from": "other"}})
	if err != nil {
		t.Fatal(err)
	}
	if guardAggregation([]bson.M{{"$facet": bson.M{"nested": bson.A{bson.Raw(raw)}}}}) == nil {
		t.Fatal("accepted raw BSON bypass")
	}
	for _, stage := range []string{"$lookup", "$unionWith", "$graphLookup", "$out", "$merge", "$documents"} {
		for _, pipeline := range []any{
			[]bson.M{{stage: "other"}},
			[]bson.M{{"$facet": bson.M{"nested": bson.A{bson.D{{Key: stage, Value: "other"}}}}}},
		} {
			if guardAggregation(pipeline) == nil {
				t.Fatalf("accepted unsafe %s", stage)
			}
		}
	}
	if err := guardAggregation([]bson.M{{"$match": bson.M{"status": "active"}}, {"$group": bson.M{"_id": "$status", "n": bson.M{"$sum": 1}}}}); err != nil {
		t.Fatal(err)
	}
}

func TestOperatorDocumentPreservation(t *testing.T) {
	for _, value := range []any{bson.D{{Key: "tags", Value: "one"}}, map[string]string{"tags": "one"}, bson.M{"tags": "one"}} {
		copied, err := copyOperatorDocument(value)
		if err != nil {
			t.Fatal(err)
		}
		if copied["tags"] != "one" {
			t.Fatal("lost caller operation")
		}
		copied["extra"] = "new"
		original, err := documentFields(value)
		if err != nil {
			t.Fatal(err)
		}
		if len(original) != 1 {
			t.Fatal("mutated caller map")
		}
	}
}
