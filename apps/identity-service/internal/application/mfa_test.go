package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- RFC 6238 interoperability test.
	"encoding/base32"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
)

const testMFAKey = "identity-mfa-test-key-that-is-long-enough"

func TestPrivilegedMFASetupVerificationAndReplayProtection(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	svc := newMFAService(t, &now)
	ctx := tenantMFAContext("upshs")

	setup, err := svc.LoginStart(ctx, "admin@upshs.edu.gh", "password123")
	if err != nil {
		t.Fatalf("start setup: %v", err)
	}
	if setup.Status != "mfa_setup_required" || setup.Secret == "" || !strings.HasPrefix(setup.OTPAuthURI, "otpauth://totp/") || setup.AccessToken != "" {
		t.Fatalf("unexpected setup response: %+v", setup)
	}
	code := testTOTPCode(t, setup.Secret, now)
	access, refresh, user, _, err := svc.CompleteMFA(ctx, setup.ChallengeToken, code, setup.Secret)
	if err != nil || access == "" || refresh == "" || user.Role != "school_admin" {
		t.Fatalf("complete setup: user=%+v err=%v", user, err)
	}
	if _, _, _, _, err := svc.CompleteMFA(ctx, setup.ChallengeToken, code, setup.Secret); err == nil {
		t.Fatal("setup challenge replay was accepted")
	}

	verify, err := svc.LoginStart(ctx, "admin@upshs.edu.gh", "password123")
	if err != nil || verify.Status != "mfa_required" || verify.Secret != "" || verify.ChallengeToken == "" {
		t.Fatalf("start verification: %+v err=%v", verify, err)
	}
	if _, _, _, _, err := svc.CompleteMFA(ctx, verify.ChallengeToken, code, ""); err == nil {
		t.Fatal("previously accepted TOTP counter was accepted")
	}

	now = now.Add(31 * time.Second)
	nextCode := testTOTPCode(t, setup.Secret, now)
	if _, _, _, _, err := svc.CompleteMFA(ctx, verify.ChallengeToken, nextCode, ""); err != nil {
		t.Fatalf("complete verification: %v", err)
	}
	if _, _, _, _, err := svc.CompleteMFA(ctx, verify.ChallengeToken, nextCode, ""); err == nil {
		t.Fatal("TOTP code replay was accepted")
	}
}

func TestPrivilegedMFAIsTenantBoundAndOrdinaryUsersContinueDirectly(t *testing.T) {
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	svc := newMFAService(t, &now)
	if _, _, _, _, err := svc.Login(tenantMFAContext("upshs"), "admin@upshs.edu.gh", "password123"); !errors.Is(err, domain.ErrMFARequired) {
		t.Fatalf("direct privileged login bypassed MFA: %v", err)
	}

	teacher, err := svc.LoginStart(tenantMFAContext("upshs"), "e.mensah@upshs.edu.gh", "password123")
	if err != nil || teacher.Status != "authenticated" || teacher.AccessToken == "" {
		t.Fatalf("teacher login: %+v err=%v", teacher, err)
	}
	setup, err := svc.LoginStart(tenantMFAContext("upshs"), "admin@upshs.edu.gh", "password123")
	if err != nil {
		t.Fatalf("admin login: %v", err)
	}
	code := testTOTPCode(t, setup.Secret, now)
	access, refresh, challengedUser, expires, err := svc.CompleteMFA(
		tenantMFAContext("other-school"), setup.ChallengeToken, code, setup.Secret,
	)
	if access != "" || refresh != "" || challengedUser.ID != "" || !expires.IsZero() {
		t.Fatalf("rejected challenge returned session material: user=%+v expires=%v", challengedUser, expires)
	}
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("cross-tenant challenge error = %v", err)
	}
}

func newMFAService(t *testing.T, now *time.Time) *Service {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("memory repository: %v", err)
	}
	repo.WithClock(func() time.Time { return *now })
	svc := NewService(repo, nil, events.NewRecordingPublisher(), []byte("test-signing-key"), time.Hour, 7*24*time.Hour,
		WithPrivilegedMFA(testMFAKey, true))
	return svc.WithClock(func() time.Time { return *now })
}

func tenantMFAContext(tenantID string) context.Context {
	return tenancy.WithActor(context.Background(), auth.Actor{TenantID: tenantID})
}

func testTOTPCode(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	var message [8]byte
	counter := now.Unix() / 30
	if counter < 0 {
		t.Fatal("test TOTP timestamp precedes Unix epoch")
	}
	binary.BigEndian.PutUint64(message[:], uint64(counter)) //nolint:gosec // the negative counter case is rejected above.
	mac := hmac.New(sha1.New, raw)                          // #nosec G401 -- RFC 6238 interoperability test.
	_, _ = mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return strconv.Itoa(int(value%1_000_000) + 1_000_000)[1:]
}
