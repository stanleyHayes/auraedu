package httpx

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeJSONResponse(t *testing.T) {
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := DecodeJSONResponse(strings.NewReader(`{"enabled":true}`), &payload); err != nil || !payload.Enabled {
		t.Fatalf("decode valid response: payload=%+v err=%v", payload, err)
	}
	if err := DecodeJSONResponse(strings.NewReader(`{"enabled":true}{"enabled":false}`), &payload); err == nil {
		t.Fatal("multiple JSON values must be rejected")
	}
	oversized := strings.NewReader(`{"padding":"` + strings.Repeat("x", int(MaxInternalJSONResponseBytes)) + `"}`)
	if err := DecodeJSONResponse(oversized, &payload); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized response error=%v", err)
	}
}
