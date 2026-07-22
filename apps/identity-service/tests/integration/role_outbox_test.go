package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/auraedu/identity-service/internal/adapters/events"
	"github.com/auraedu/identity-service/internal/adapters/postgres"
	"github.com/auraedu/identity-service/internal/application"
	identitydb "github.com/auraedu/identity-service/internal/db"
	"github.com/auraedu/identity-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/testkit"
	"github.com/google/uuid"
)

func TestRoleAndPermissionChangesCommitWithOutbox(t *testing.T) {
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
	direct := events.NewRecordingPublisher()
	svc := application.NewService(repo, nil, direct, []byte("integration-signing-key"), time.Hour, 7*24*time.Hour)
	actor := auth.Actor{
		UserID: "school-admin", TenantID: "upshs", Role: "school_admin",
		Permissions: []string{"users.create", "users.read", "users.update", "roles.assign", "students.read", "staff.read"},
	}
	user, err := svc.CreateUser(ctx, actor, application.CreateUserInput{
		Email: "teacher@upshs.example", Name: "Teacher", Role: "teacher",
		Permissions: []string{"students.read"},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := repo.SaveRefreshToken(ctx, user.ID, "pre-role-change", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed pre-role session: %v", err)
	}
	role := "principal"
	permissions := []string{"students.read", "staff.read"}
	updated, err := svc.UpdateUser(ctx, actor, user.ID, application.UpdateUserInput{
		Role: &role, Permissions: &permissions,
	})
	if err != nil {
		t.Fatalf("update authorization: %v", err)
	}
	if updated.Role != role || len(updated.Permissions) != 2 {
		t.Fatalf("updated user=%+v", updated)
	}
	if _, err := repo.RotateRefreshToken(ctx, "pre-role-change", "post-role-change", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("role mutation did not revoke prior session: %v", err)
	}
	if len(direct.Events) != 0 {
		t.Fatalf("postgres mutation must not use direct publisher: %+v", direct.Events)
	}
	if err := repo.SaveRefreshToken(ctx, user.ID, "concurrent-rotation", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed concurrent session: %v", err)
	}
	consumeResults := make(chan error, 2)
	var consumeWG sync.WaitGroup
	for index := range 2 {
		consumeWG.Add(1)
		go func(successor string) {
			defer consumeWG.Done()
			_, consumeErr := repo.RotateRefreshToken(ctx, "concurrent-rotation", successor, time.Now().Add(time.Hour))
			consumeResults <- consumeErr
		}(fmt.Sprintf("concurrent-successor-%d", index))
	}
	consumeWG.Wait()
	close(consumeResults)
	consumed, replayed := 0, 0
	for consumeErr := range consumeResults {
		switch {
		case consumeErr == nil:
			consumed++
		case errors.Is(consumeErr, domain.ErrExpiredToken):
			replayed++
		default:
			t.Fatalf("unexpected consume result: %v", consumeErr)
		}
	}
	if consumed != 1 || replayed != 1 {
		t.Fatalf("atomic consume results: consumed=%d replayed=%d", consumed, replayed)
	}
	for _, successor := range []string{"concurrent-successor-0", "concurrent-successor-1"} {
		if _, err := repo.RotateRefreshToken(ctx, successor, successor+"-probe", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
			t.Fatalf("replayed family left successor %q active: %v", successor, err)
		}
	}
	if err := repo.SaveRefreshToken(ctx, user.ID, "logout-race-old", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed logout-race family: %v", err)
	}
	if _, err := repo.RotateRefreshToken(ctx, "logout-race-old", "logout-race-current", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("prepare logout-race current token: %v", err)
	}
	raced := make(chan struct {
		operation string
		err       error
	}, 2)
	var raceWG sync.WaitGroup
	raceWG.Add(2)
	go func() {
		defer raceWG.Done()
		_, rotateErr := repo.RotateRefreshToken(ctx, "logout-race-current", "logout-race-child", time.Now().Add(time.Hour))
		raced <- struct {
			operation string
			err       error
		}{operation: "rotate", err: rotateErr}
	}()
	go func() {
		defer raceWG.Done()
		raced <- struct {
			operation string
			err       error
		}{operation: "logout", err: repo.RevokeRefreshFamily(ctx, "logout-race-old")}
	}()
	raceWG.Wait()
	close(raced)
	for result := range raced {
		if result.operation == "logout" && result.err != nil {
			t.Fatalf("family logout failed: %v", result.err)
		}
		if result.operation == "rotate" && result.err != nil && !errors.Is(result.err, domain.ErrExpiredToken) {
			t.Fatalf("unexpected concurrent rotate result: %v", result.err)
		}
	}
	if _, err := repo.RotateRefreshToken(ctx, "logout-race-child", "logout-race-probe", time.Now().Add(time.Hour)); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("logout/rotation race left child active: %v", err)
	}

	pending, err := repo.ClaimPending(ctx, 10)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(pending) != 1 || pending[0].EventType != "user.role_changed.v1" || pending[0].TenantID != "upshs" {
		t.Fatalf("outbox=%+v", pending)
	}
	var payload map[string]any
	if err := json.Unmarshal(pending[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["user_id"] != user.ID || payload["previous_role"] != "teacher" || payload["new_role"] != role {
		t.Fatalf("payload=%v", payload)
	}
	for _, forbidden := range []string{"email", "name", "password", "tenant_id"} {
		if _, found := payload[forbidden]; found {
			t.Fatalf("role event leaked %s", forbidden)
		}
	}
	if err := repo.MarkPublished(ctx, pending[0].ID); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	// Exact replay is a no-op rather than a second authorization event.
	if _, err := svc.AssignRole(ctx, actor, user.ID, role, permissions); err != nil {
		t.Fatalf("idempotent assignment: %v", err)
	}
	if err := repo.SaveRefreshToken(ctx, user.ID, "rollback-role-change", uuid.NewString(), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed rollback session: %v", err)
	}
	pending, err = repo.ClaimPending(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("idempotent assignment emitted event: pending=%+v err=%v", pending, err)
	}

	// Removing the outbox makes the authorization mutation fail and roll back.
	if _, err := pool.Exec(ctx, `DROP TABLE identity_outbox`); err != nil {
		t.Fatalf("drop outbox for atomicity probe: %v", err)
	}
	if _, err := svc.AssignRole(ctx, actor, user.ID, "academic_head", []string{"users.read"}); err == nil {
		t.Fatal("role assignment must fail without its outbox")
	}
	if gotUserID, err := repo.RotateRefreshToken(ctx, "rollback-role-change", "rollback-successor", time.Now().Add(time.Hour)); err != nil || gotUserID != user.ID {
		t.Fatalf("failed authorization transaction revoked session: user=%q err=%v", gotUserID, err)
	}
	after, err := svc.GetUser(ctx, actor, user.ID)
	if err != nil {
		t.Fatalf("read rolled-back user: %v", err)
	}
	if after.Role != role {
		t.Fatalf("role committed without event: %q", after.Role)
	}
	if after.Status != domain.StatusActive {
		t.Fatalf("unexpected user state after rollback: %+v", after)
	}
}
