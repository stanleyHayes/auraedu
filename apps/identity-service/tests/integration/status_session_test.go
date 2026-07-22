package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/application"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/identity-service/internal/tenancy"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestAccountStatusChangeAndSessionRevocationAreAtomic(t *testing.T) {
	ctx := context.Background()
	tdb := testkit.NewPostgres(ctx, t, "")
	pool, err := identitydb.Open(ctx, tdb.DSN)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()
	if err := identitydb.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := postgres.NewRepository(pool)
	svc := application.NewService(repo, nil, events.NewRecordingPublisher(), []byte("status-session-signing-key"), time.Hour, 7*24*time.Hour)
	actor := auth.Actor{
		UserID: "platform-admin", PlatformAdmin: true, Role: auth.RolePlatformSuperAdmin,
		Permissions: []string{"users.create", "users.update", "roles.assign"},
	}
	create := func(email string) domain.User {
		t.Helper()
		user, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
			TenantID: "upshs", Email: email, Name: "Status Test", Role: "teacher",
		})
		if err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
		return user
	}

	lockedUser := create("lock-success@upshs.example")
	if err := repo.SaveRefreshToken(ctx, lockedUser.ID, "lock-success-token", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed lock session: %v", err)
	}
	locked := domain.StatusLocked
	if _, err := svc.UpdateUser(ctx, actor, lockedUser.ID, application.UpdateUserInput{Status: &locked}); err != nil {
		t.Fatalf("lock user: %v", err)
	}
	if _, err := repo.RotateRefreshToken(ctx, "lock-success-token", "lock-success-child", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("lock committed without revoking session: %v", err)
	}

	rollbackUser := create("lock-rollback@upshs.example")
	if err := repo.SaveRefreshToken(ctx, rollbackUser.ID, "lock-rollback-token", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed rollback session: %v", err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE refresh_tokens`); err != nil {
		t.Fatalf("remove refresh table for rollback probe: %v", err)
	}
	if _, err := svc.UpdateUser(ctx, actor, rollbackUser.ID, application.UpdateUserInput{Status: &locked}); err == nil {
		t.Fatal("status mutation must fail when session revocation cannot commit")
	}
	unchanged, err := repo.GetUser(contextWithPlatformActor(ctx), rollbackUser.ID)
	if err != nil {
		t.Fatalf("read rollback user: %v", err)
	}
	if unchanged.Status != domain.StatusActive {
		t.Fatalf("status committed without session revocation: %+v", unchanged)
	}
}

func contextWithPlatformActor(ctx context.Context) context.Context {
	return tenancy.WithActor(ctx, auth.Actor{PlatformAdmin: true})
}
