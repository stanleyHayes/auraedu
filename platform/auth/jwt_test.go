package auth

import (
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
	token, _ := Sign(Claims{Subject: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}, key)
	if _, err := Verify(token, []byte("attacker-key"), time.Now()); err != ErrInvalidToken {
		t.Fatalf("wrong key should be ErrInvalidToken, got %v", err)
	}
	if _, err := Verify(token+"tamper", key, time.Now()); err != ErrInvalidToken {
		t.Fatalf("tampered signature should be ErrInvalidToken, got %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	key := []byte("test-signing-key")
	token, _ := Sign(Claims{Subject: "u1", ExpiresAt: 1000}, key)
	if _, err := Verify(token, key, time.Unix(2000, 0)); err != ErrExpiredToken {
		t.Fatalf("expired token should be ErrExpiredToken, got %v", err)
	}
}

func TestPlatformAdminActor(t *testing.T) {
	got := Claims{Subject: "s1", Role: RolePlatformSuperAdmin}.Actor()
	if !got.PlatformAdmin || !got.CanAccessTenant("any-tenant") {
		t.Fatal("platform super admin should access any tenant")
	}
}
