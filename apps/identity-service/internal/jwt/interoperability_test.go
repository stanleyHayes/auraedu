package jwt

import (
	"testing"
	"time"

	"github.com/auraedu/platform/auth"
)

// This signs using the real Identity issuer and verifies using the gateway's
// shared implementation, so claim-name drift cannot silently discard grants.
func TestAccessTokenInteroperability(t *testing.T) {
	now := time.Now()
	key := []byte("identity-gateway-interoperability-test-key")
	token, err := Sign(Claims{
		Subject: "user-one", UserID: "user-one", TenantID: "school-one", Role: "school_admin",
		Permissions: []string{"students.read"}, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := auth.Verify(token, key, now)
	if err != nil {
		t.Fatal(err)
	}
	actor := verified.Actor()
	if actor.UserID != "user-one" || actor.TenantID != "school-one" || actor.Role != "school_admin" {
		t.Fatalf("identity claims were changed: %+v", actor)
	}
	if !actor.Has("students.read") || actor.Has("students.create") || actor.PlatformAdmin {
		t.Fatal("gateway changed the issuer's permission grants")
	}
	if _, err := auth.Verify(token, key, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired Identity token accepted")
	}
}
