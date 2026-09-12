package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	key := []byte("test-signing-key")
	claims := Claims{
		Subject: "u1", TenantID: "upshs", Role: "teacher",
		Permissions: []string{"attendance.mark"},
		IssuedAt:    1000, ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := Sign(claims, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := Verify(token, key, time.Now())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Subject != "u1" || got.TenantID != "upshs" || got.Role != "teacher" {
		t.Fatalf("claims mismatch: %+v", got)
	}
	actor := got.Actor()
	if actor.UserID != "u1" || !actor.CanAccessTenant("upshs") || !actor.Has("attendance.mark") {
		t.Fatalf("actor mismatch: %+v", actor)
	}
}

func TestVerifyRejectsWrongKeyAndTamper(t *testing.T) {
	key := []byte("test-signing-key")
	token, err := Sign(Claims{Subject: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := Verify(token, []byte("attacker-key"), time.Now()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong key should be ErrInvalidToken, got %v", err)
	}
	if _, err := Verify(token+"tamper", key, time.Now()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered signature should be ErrInvalidToken, got %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	key := []byte("test-signing-key")
	token, err := Sign(Claims{Subject: "u1", ExpiresAt: 1000}, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := Verify(token, key, time.Unix(2000, 0)); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expired token should be ErrExpiredToken, got %v", err)
	}
}

func TestPlatformAdminActor(t *testing.T) {
	got := Claims{Subject: "s1", Role: RolePlatformSuperAdmin}.Actor()
	if !got.PlatformAdmin || !got.CanAccessTenant("any-tenant") {
		t.Fatal("platform super admin should access any tenant")
	}
}

func TestIdentityAccessPermissionClaims(t *testing.T) {
	key := []byte("identity-interoperability-key")
	// Mirrors Identity's access payload, including its extra claims. This is
	// deliberately not marshaled through platform Claims.
	raw := `{"sub":"user-1","user_id":"user-1","tenant_id":"school-a","role":"teacher","permissions":["students.read"],"features_hash":"snapshot","typ":"access","iat":1000,"exp":2000}`
	token := signedPayload(raw, key)
	claims, err := Verify(token, key, time.Unix(1500, 0))
	if err != nil {
		t.Fatal(err)
	}
	actor := claims.Actor()
	if !actor.Has("students.read") || actor.Has("students.delete") || !actor.CanAccessTenant("school-a") || actor.CanAccessTenant("school-b") {
		t.Fatalf("unexpected access grants: %+v", actor)
	}
	if _, err := Verify(token, []byte("other-key"), time.Unix(1500, 0)); !errors.Is(err, ErrInvalidToken) {
		t.Fatal(err)
	}
	if _, err := Verify(token, key, time.Unix(2000, 0)); !errors.Is(err, ErrExpiredToken) {
		t.Fatal(err)
	}
}

func signedPayload(raw string, key []byte) string {
	input := jwtHeader + "." + base64.RawURLEncoding.EncodeToString([]byte(raw))
	return input + "." + sign(input, key)
}

func TestPermissionClaimCompatibility(t *testing.T) {
	cases := []struct {
		name, fields   string
		grant, invalid bool
	}{
		{"canonical", `"permissions":["students.read"]`, true, false},
		{"legacy", `"perms":["students.read"]`, true, false},
		{"matching aliases", `"permissions":["students.read"],"perms":["students.read"]`, true, false},
		{"conflicting aliases", `"permissions":["students.read"],"perms":["students.delete"]`, false, true},
		{"empty canonical cannot regain legacy grants", `"permissions":[],"perms":["students.read"]`, false, true},
		{"null canonical cannot regain legacy grants", `"permissions":null,"perms":["students.read"]`, false, true},
		{"canonical wrong type", `"permissions":"students.read"`, false, true},
		{"canonical wrong element", `"permissions":[4]`, false, true},
		{"malformed legacy", `"permissions":["students.read"],"perms":true`, false, true},
		{"empty canonical", `"permissions":[]`, false, false},
		{"null canonical", `"permissions":null`, false, false},
		{"no permissions", `"typ":"access"`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"sub":"user-1","tenant_id":"school-a","role":"teacher","exp":2000,` + tc.fields + `}`
			claims, err := Verify(signedPayload(raw, []byte("key")), []byte("key"), time.Unix(1500, 0))
			if tc.invalid {
				if !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("want invalid, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if claims.Actor().Has("students.read") != tc.grant {
				t.Fatalf("unexpected grants: %+v", claims.Permissions)
			}
		})
	}
}

func TestSignUsesCanonicalPermissionClaim(t *testing.T) {
	token, err := Sign(Claims{Subject: "user-1", Permissions: []string{"students.read"}}, []byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["permissions"]; !ok {
		t.Fatal("canonical permissions missing")
	}
	if _, ok := fields["perms"]; ok {
		t.Fatal("legacy permissions emitted")
	}
}
