package http

import (
	"encoding/json"
	"testing"
)

func TestNullableStringPatchDistinguishesOmittedNullAndValue(t *testing.T) {
	omitted, err := nullableStringPatch(nil)
	if err != nil || omitted != nil {
		t.Fatalf("omitted=%v err=%v", omitted, err)
	}
	cleared, err := nullableStringPatch(json.RawMessage("null"))
	if err != nil || cleared == nil || *cleared != "" {
		t.Fatalf("cleared=%v err=%v", cleared, err)
	}
	value, err := nullableStringPatch(json.RawMessage(`"33333333-3333-4333-8333-333333333333"`))
	if err != nil || value == nil || *value != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("value=%v err=%v", value, err)
	}
	if _, err := nullableStringPatch(json.RawMessage("42")); err == nil {
		t.Fatal("non-string patch value must be rejected")
	}
}
