package mongo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/ports"
	it "github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	pm "github.com/auraedu/platform/mongo"
	"github.com/testcontainers/testcontainers-go"
	tcm "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func testContext(tenant string) context.Context {
	return it.WithActor(context.Background(), auth.Actor{TenantID: tenant})
}
func TestMongoSecurityAtomicity(t *testing.T) {
	if testing.Short() {
		t.Skip("real Mongo integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ctr, err := tcm.Run(ctx, "mongo:8.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	uri, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	database, err := pm.Open(ctx, pm.Config{URI: uri, Database: "identity_security_test"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	r := NewRepository(database)
	if err = r.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	// Two repository instances ensure guarantees are in Mongo, not a local lock.
	second := NewRepository(database)
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	cred, err := domain.NewCredential("initial-password-123")
	must(t, err)
	user := func(tenant, email string) string {
		t.Helper()
		id, err := r.CreateUser(testContext(tenant), domain.User{TenantID: tenant, Email: email, Role: "teacher", Status: domain.StatusActive}, cred)
		must(t, err)
		return id
	}
	a := user("school-a", "same@example.test")
	b := user("school-b", "same@example.test")
	t.Run("tenant isolation and platform login", func(t *testing.T) {
		if _, err := r.GetUser(testContext("school-b"), a); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant read: %v", err)
		}
		if err := r.UpdateUser(testContext("school-b"), a, domain.User{Name: "intrusion"}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("cross-tenant write: %v", err)
		}
		if _, err := r.ListUsers(ctx); !errors.Is(err, pm.ErrTenantRequired) {
			t.Fatalf("unscoped query: %v", err)
		}
		u, _, found, err := r.FindByEmail(testContext("school-b"), "SAME@example.test")
		must(t, err)
		if !found || u.ID != b {
			t.Fatal("email lookup escaped tenant")
		}
		admin := it.WithActor(ctx, auth.Actor{PlatformAdmin: true})
		id, err := r.CreateUser(admin, domain.User{Email: "platform@example.test", Role: auth.RolePlatformSuperAdmin, Status: domain.StatusActive}, cred)
		must(t, err)
		u, _, found, err = r.FindByEmail(testContext("school-a"), "platform@example.test")
		must(t, err)
		if !found || u.ID != id || u.TenantID != "" {
			t.Fatal("platform identity fallback")
		}
	})
	t.Run("MFA counter concurrent replay", func(t *testing.T) {
		must(t, r.SaveMFA(ctx, a, []byte("encrypted-secret"), 10))
		if err := r.SaveMFA(ctx, a, []byte("replacement"), 11); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("MFA replacement: %v", err)
		}
		var accepted atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				repo := r
				if i%2 == 1 {
					repo = second
				}
				err := repo.AdvanceMFACounter(ctx, a, 11)
				if err == nil {
					accepted.Add(1)
				} else if !errors.Is(err, domain.ErrInvalidCredentials) {
					t.Errorf("counter: %v", err)
				}
			}(i)
		}
		wg.Wait()
		if accepted.Load() != 1 {
			t.Fatalf("accepted replay %d times", accepted.Load())
		}
	})
	t.Run("refresh concurrent rotation revokes successor", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, a, "old", "family", time.Now().Add(time.Hour)))
		var accepted atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				repo := r
				if i == 1 {
					repo = second
				}
				_, err := repo.RotateRefreshToken(ctx, "old", fmt.Sprintf("new-%d", i), time.Now().Add(time.Hour))
				if err == nil {
					accepted.Add(1)
				} else if !errors.Is(err, domain.ErrExpiredToken) {
					t.Errorf("rotate: %v", err)
				}
			}(i)
		}
		wg.Wait()
		if accepted.Load() != 1 {
			t.Fatalf("concurrent rotation accepted %d", accepted.Load())
		}
		for i := 0; i < 2; i++ {
			if _,
				err := r.RotateRefreshToken(ctx,
				fmt.Sprintf("new-%d",
					i),
				fmt.Sprintf("later-%d",
					i),
				time.Now().Add(time.Hour)); !errors.Is(err,
				domain.ErrExpiredToken) {
				t.Fatalf("replay did not revoke successor: %v", err)
			}
		}
	})
	t.Run("reset supersession atomic password and revocation", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, b, "reset-session", "reset-family", time.Now().Add(time.Hour)))
		must(t, r.SavePasswordResetToken(ctx, "school-b", b, "reset-old", time.Now().Add(time.Hour)))
		must(t, r.SavePasswordResetToken(ctx, "school-b", b, "reset-current", time.Now().Add(time.Hour)))
		replacement, err := domain.NewCredential("replacement-password-123")
		must(t, err)
		for _, pair := range [][2]string{{"reset-old", "school-b"}, {"reset-current", "school-a"}} {
			if err := r.ResetPasswordWithToken(ctx, pair[0], pair[1], replacement); !errors.Is(err, domain.ErrExpiredToken) {
				t.Fatalf("invalid reset accepted: %v", err)
			}
		}
		must(t, r.ResetPasswordWithToken(ctx, "reset-current", "school-b", replacement))
		if err := r.ResetPasswordWithToken(ctx, "reset-current", "school-b", cred); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("reset replay accepted: %v", err)
		}
		_, stored, _, err := r.FindByEmail(testContext("school-b"), "same@example.test")
		must(t, err)
		if !stored.Verify("replacement-password-123") {
			t.Fatal("credential lost")
		}
		if _, err := r.RotateRefreshToken(ctx, "reset-session", "reset-next", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("reset session survived: %v", err)
		}
	})
	t.Run("invite first credential wins and retries match", func(t *testing.T) {
		must(t, r.SaveInvite(ctx, "school-a", "invite@example.test", "teacher", []string{"students.read"}, "invite-old", nil, time.Now().Add(time.Hour)))
		must(t, r.SaveInvite(ctx, "school-a", "invite@example.test", "teacher", []string{"students.read"}, "invite-current", nil, time.Now().Add(time.Hour)))
		if _, err := r.InspectInvite(ctx, "invite-old"); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("superseded invite: %v", err)
		}
		var accepted atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				password := fmt.Sprintf("invite-password-%d", i)
				c, err := domain.NewCredential(password)
				if err != nil {
					t.Error(err)
					return
				}
				_, err = second.AcceptInviteWithCredential(ctx, "invite-current", "Name", password, c)
				if err == nil {
					accepted.Add(1)
				} else if !errors.Is(err, domain.ErrExpiredToken) {
					t.Errorf("invite: %v", err)
				}
			}(i)
		}
		wg.Wait()
		if accepted.Load() != 1 {
			t.Fatalf("credential installed %d times", accepted.Load())
		}
		u, c, _, err := r.FindByEmail(testContext("school-a"), "invite@example.test")
		must(t, err)
		password := "invite-password-0"
		if !c.Verify(password) {
			password = "invite-password-1"
		}
		retry, err := r.AcceptInviteWithCredential(ctx, "invite-current", "Retry", password, c)
		must(t, err)
		if retry.ID != u.ID {
			t.Fatal("retry created second identity")
		}
	})
	t.Run("role sessions outbox atomic and durable", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, a, "role-session", "role-family", time.Now().Add(time.Hour)))
		must(t,
			r.UpdateUserWithRoleChange(testContext("school-a"),
				a,
				domain.User{Role: "staff"},
				ports.RoleChangeEvent{TenantID: "school-a",
					UserID:       a,
					PreviousRole: "teacher",
					NewRole:      "staff"}))
		if _, err := r.RotateRefreshToken(ctx, "role-session", "role-next", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("role session survived: %v", err)
		}
		pending, err := second.ClaimPending(ctx, 10)
		must(t, err)
		if len(pending) != 1 || pending[0].EventType != "user.role_changed.v1" {
			t.Fatalf("outbox: %+v", pending)
		}
		blocked, err := r.ClaimPending(ctx, 10)
		must(t, err)
		if len(blocked) != 0 {
			t.Fatal("outbox double claimed")
		}
		must(t, second.MarkPublished(ctx, pending[0].ID))
		empty, err := r.ClaimPending(ctx, 10)
		must(t, err)
		if len(empty) != 0 {
			t.Fatal("published event remained")
		}
	})
	t.Run("cleanup retains replay ancestor while family lives", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, a, "cleanup-old", "cleanup-family", time.Now().Add(-2*time.Hour)))
		must(t, r.SaveRefreshToken(ctx, a, "cleanup-live", "cleanup-family", time.Now().Add(time.Hour)))
		_,
			err := r.CleanupAuthArtifacts(ctx,
			ports.AuthRetentionCutoffs{RefreshFamiliesBefore: time.Now().Add(-time.Hour),
				PasswordResetsBefore: time.Now().Add(-time.Hour),
				InvitesBefore:        time.Now().Add(-time.Hour),
				BatchSize:            100})
		must(t, err)
		if _, err := r.FindRefreshToken(ctx, "cleanup-old"); err != nil {
			t.Fatalf("live-family replay ancestor removed: %v", err)
		}
		if _, err := r.RotateRefreshToken(ctx, "cleanup-old", "cleanup-new", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("expired ancestor accepted: %v", err)
		}
		if _, err := r.RotateRefreshToken(ctx, "cleanup-live", "cleanup-next", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("cleanup lost replay revocation: %v", err)
		}
	})
	t.Run("failed role commit keeps session and emits no event", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, b, "rollback-session", "rollback-family", time.Now().Add(time.Hour)))
		must(t,
			database.Database().RunCommand(ctx,
				bson.D{{Key: "collMod",
					Value: collectionName},
					{Key: "validator",
						Value: bson.M{"user.role": bson.M{"$ne": "forbidden"}}}}).Err())
		err := r.UpdateUserWithRoleChange(testContext("school-b"),
			b,
			domain.User{Role: "forbidden"},
			ports.RoleChangeEvent{TenantID: "school-b",
				UserID:       b,
				PreviousRole: "teacher",
				NewRole:      "forbidden"})
		if err == nil {
			t.Fatal("injected validator failure accepted")
		}
		must(t, database.Database().RunCommand(ctx, bson.D{{Key: "collMod", Value: collectionName}, {Key: "validator", Value: bson.M{}}}).Err())
		u, err := r.GetUser(testContext("school-b"), b)
		must(t, err)
		if u.Role != "teacher" {
			t.Fatal("failed role mutation persisted")
		}
		pending, err := r.ClaimPending(ctx, 10)
		must(t, err)
		if len(pending) != 0 {
			t.Fatal("failed mutation emitted event")
		}
		_, err = r.RotateRefreshToken(ctx, "rollback-session", "rollback-successor", time.Now().Add(time.Hour))
		must(t, err)
	})
	t.Run("status and revoke atomically survive reopen", func(t *testing.T) {
		must(t, r.SaveRefreshToken(ctx, b, "status-session", "status-family", time.Now().Add(time.Hour)))
		must(t, r.UpdateUserAndRevokeSessions(testContext("school-b"), b, domain.User{Status: domain.StatusInactive}))
		u, err := second.GetUser(testContext("school-b"), b)
		must(t, err)
		if u.Status != domain.StatusInactive {
			t.Fatal("status did not persist")
		}
		if _, err := second.RotateRefreshToken(ctx, "status-session", "status-successor", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("disabled user retained session: %v", err)
		}
	})
	t.Run("onboarding lease recovery and stale owner fence", func(t *testing.T) {
		first := WithOnboardingLease(ctx)
		next := WithOnboardingLease(ctx)
		claimed, err := r.ClaimOnboarding(first, "event-1", "tenant.onboarding_approved.v1", "school-a")
		must(t, err)
		if !claimed {
			t.Fatal("first claim denied")
		}
		if _, err := second.ClaimOnboarding(next, "event-1", "tenant.onboarding_approved.v1", "school-a"); err == nil {
			t.Fatal("active lease must be retried, not acknowledged")
		}
		oldNow := second.now
		second.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
		defer func() { second.now = oldNow }()
		claimed, err = second.ClaimOnboarding(next, "event-1", "tenant.onboarding_approved.v1", "school-a")
		must(t, err)
		if !claimed {
			t.Fatal("expired lease not recovered")
		}
		if err := r.CompleteOnboarding(first, "event-1", "school-a"); err == nil {
			t.Fatal("stale worker completed successor lease")
		}
		must(t, r.ReleaseOnboarding(first, "event-1", "school-a"))
		must(t, second.CompleteOnboarding(next, "event-1", "school-a"))
		claimed, err = r.ClaimOnboarding(WithOnboardingLease(ctx), "event-1", "tenant.onboarding_approved.v1", "school-a")
		must(t, err)
		if claimed {
			t.Fatal("completed event repeated")
		}
	})

	t.Run("delete recreate retains undelivered events", func(t *testing.T) {
		oldID := user("school-delete", "recreate@example.test")
		must(t,
			r.UpdateUserWithRoleChange(testContext("school-delete"),
				oldID,
				domain.User{Role: "staff"},
				ports.RoleChangeEvent{TenantID: "school-delete",
					UserID:       oldID,
					PreviousRole: "teacher",
					NewRole:      "staff"}))
		must(t, r.SaveRefreshToken(ctx, oldID, "delete-refresh", "delete-family", time.Now().Add(time.Hour)))
		must(t, r.DeleteUser(testContext("school-delete"), oldID))
		nextID := user("school-delete", "recreate@example.test")
		if oldID == nextID {
			t.Fatal("recreated identity reused old user ID")
		}
		if _, err := r.RotateRefreshToken(ctx, "delete-refresh", "delete-next", time.Now().Add(time.Hour)); err == nil {
			t.Fatal("deleted identity refresh survived")
		}
		_, _, found, err := r.FindByEmail(testContext("school-delete"), "recreate@example.test")
		must(t, err)
		if !found {
			t.Fatal("new account is not discoverable")
		}
		pending, err := second.ClaimPending(ctx, 100)
		must(t, err)
		retained := false
		for _, event := range pending {
			if event.TenantID == "school-delete" && event.EventType == "user.role_changed.v1" {
				retained = true
			}
		}
		if !retained {
			t.Fatal("delete/recreate lost undelivered event")
		}
	})
}
