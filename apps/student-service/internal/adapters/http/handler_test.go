package http

import (
	"encoding/json"
	"testing"
)

func TestNullableStringPatchDistinguishesOmittedNullAndValue(t *testing.T) {
	if value, err := nullableStringPatch(nil); err != nil || value != nil {
		t.Fatalf("omitted=%v err=%v", value, err)
	}
	cleared, err := nullableStringPatch(json.RawMessage("null"))
	if err != nil || cleared == nil || *cleared != "" {
		t.Fatalf("cleared=%v err=%v", cleared, err)
	}
	linked, err := nullableStringPatch(json.RawMessage(`"33333333-3333-4333-8333-333333333333"`))
	if err != nil || linked == nil || *linked == "" {
		t.Fatalf("linked=%v err=%v", linked, err)
	}
	if _, err := nullableStringPatch(json.RawMessage("42")); err == nil {
		t.Fatal("non-string user_id must be rejected")
	}
}
