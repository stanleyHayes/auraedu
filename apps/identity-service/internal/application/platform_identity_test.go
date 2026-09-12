package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/memory"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
)

// Platform identities are tenantless by design (AURA-4.17). Recovery and the
// identity directory therefore accept an empty tenant, which must never widen
// into cross-tenant reach: a tenantless caller may only touch tenantless
// records, and a school caller may only touch its own.

const platformAdminEmail = "super@auraedu.dev"
const schoolAdminEmail = "admin@upshs.edu.gh"
const replacementPassword = "platform-recovery-2026"

type capturingNotifier struct {
	mu       sync.Mutex
	messages []deliveredMessage
}

type deliveredMessage struct {
	tenantID  string
	recipient string
	template  string
	token     string
}

func (n *capturingNotifier) Deliver(_ context.Context, tenantID, recipient, template string, data map[string]any) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	token, _ := data["reset_token"].(string)
	n.messages = append(n.messages, deliveredMessage{
		tenantID: tenantID, recipient: recipient, template: template, token: token,
	})
	return nil
}

func (n *capturingNotifier) only(t *testing.T) deliveredMessage {
	t.Helper()
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Fatalf("expected exactly one delivered recovery message, got %d: %+v", len(n.messages), n.messages)
	}
	return n.messages[0]
}

func (n *capturingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.messages)
}

func newPlatformIdentityService(t *testing.T) (*Service, *capturingNotifier) {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("memory repository: %v", err)
	}
	notifier := &capturingNotifier{}
	svc := NewService(
		repo,
		newRecordingSessionStore(),
		events.NewRecordingPublisher(),
		[]byte("platform-identity-signing-key"),
		time.Hour,
		7*24*time.Hour,
		WithTransactionalNotifier(notifier),
	)
	return svc, notifier
}

func tenantContext(tenantID string) context.Context {
	return tenancy.WithActor(context.Background(), auth.Actor{TenantID: tenantID})
}

func platformContext() context.Context { return tenantContext("") }

func TestPlatformAdminRecoversTenantlessCredential(t *testing.T) {
	svc, notifier := newPlatformIdentityService(t)

	if err := svc.RequestPasswordReset(platformContext(), platformAdminEmail); err != nil {
		t.Fatalf("platform recovery request: %v", err)
	}

	message := notifier.only(t)
	if message.tenantID != "" || message.recipient != platformAdminEmail {
		t.Fatalf("platform recovery must stay tenantless and addressed to its owner: %+v", message)
	}
	if message.token == "" {
		t.Fatal("platform recovery delivered no reset token")
	}

	if err := svc.ResetPassword(platformContext(), message.token, replacementPassword); err != nil {
		t.Fatalf("platform reset should succeed without a tenant: %v", err)
	}
}

func TestTenantlessRecoveryCannotTargetSchoolIdentity(t *testing.T) {
	svc, notifier := newPlatformIdentityService(t)

	if err := svc.RequestPasswordReset(platformContext(), schoolAdminEmail); err != nil {
		t.Fatalf("tenantless request for a school identity must fail silently, got: %v", err)
	}
	if notifier.count() != 0 {
		t.Fatalf("tenantless recovery issued a token for a school identity: %+v", notifier.messages)
	}
}

func TestSchoolRecoveryCannotTargetPlatformIdentity(t *testing.T) {
	svc, notifier := newPlatformIdentityService(t)

	if err := svc.RequestPasswordReset(tenantContext("upshs"), platformAdminEmail); err != nil {
		t.Fatalf("school request for a platform identity must fail silently, got: %v", err)
	}
	if notifier.count() != 0 {
		t.Fatalf("school recovery issued a token for the platform identity: %+v", notifier.messages)
	}
}

func TestPlatformResetTokenIsRejectedUnderSchoolTenantContext(t *testing.T) {
	svc, notifier := newPlatformIdentityService(t)

	if err := svc.RequestPasswordReset(platformContext(), platformAdminEmail); err != nil {
		t.Fatalf("platform recovery request: %v", err)
	}
	token := notifier.only(t).token

	if err := svc.ResetPassword(tenantContext("upshs"), token, replacementPassword); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("a school tenant consumed a platform reset token: %v", err)
	}
	// The rejected attempt must not have burned the owner's token.
	if err := svc.ResetPassword(platformContext(), token, replacementPassword); err != nil {
		t.Fatalf("platform owner lost its reset token to a rejected cross-tenant attempt: %v", err)
	}
}

func TestSchoolResetTokenIsRejectedWithoutTenantContext(t *testing.T) {
	svc, notifier := newPlatformIdentityService(t)

	if err := svc.RequestPasswordReset(tenantContext("upshs"), schoolAdminEmail); err != nil {
		t.Fatalf("school recovery request: %v", err)
	}
	token := notifier.only(t).token

	if err := svc.ResetPassword(platformContext(), token, replacementPassword); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("a tenantless caller consumed a school reset token: %v", err)
	}
	if err := svc.ResetPassword(tenantContext("upshs"), token, replacementPassword); err != nil {
		t.Fatalf("school owner lost its reset token to a rejected tenantless attempt: %v", err)
	}
}

func TestIdentityDirectoryRequiresTenantOrPlatformScope(t *testing.T) {
	svc, _ := newPlatformIdentityService(t)

	// A school actor that reaches the directory without resolved tenant context
	// must be denied rather than fall through to every tenant.
	tenantless := auth.Actor{UserID: "u-admin", Role: "school_admin", Permissions: []string{PermUsersRead}}
	if _, err := svc.ListUsers(context.Background(), tenantless); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("tenantless school actor was not denied the identity directory: %v", err)
	}

	scoped := auth.Actor{UserID: "u-admin", TenantID: "upshs", Role: "school_admin", Permissions: []string{PermUsersRead}}
	scopedUsers, err := svc.ListUsers(context.Background(), scoped)
	if err != nil {
		t.Fatalf("scoped school listing: %v", err)
	}
	if len(scopedUsers) == 0 {
		t.Fatal("scoped school listing returned nothing")
	}
	for _, u := range scopedUsers {
		if u.TenantID != "upshs" {
			t.Fatalf("school listing leaked an out-of-tenant identity: %+v", u)
		}
	}

	platform := auth.Actor{UserID: "u-super", Role: auth.RolePlatformSuperAdmin, PlatformAdmin: true}
	platformUsers, err := svc.ListUsers(context.Background(), platform)
	if err != nil {
		t.Fatalf("platform listing: %v", err)
	}
	if len(platformUsers) <= len(scopedUsers) {
		t.Fatalf("platform listing did not span more than one tenant: platform=%d scoped=%d", len(platformUsers), len(scopedUsers))
	}
}
