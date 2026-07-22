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

type recordingSessionStore struct {
	mu      sync.Mutex
	entries map[string]string
	findErr error
}

func newRecordingSessionStore() *recordingSessionStore {
	return &recordingSessionStore{entries: make(map[string]string)}
}

func (s *recordingSessionStore) Save(_ context.Context, _, userID, tokenHash string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[tokenHash] = userID
	return nil
}

func (s *recordingSessionStore) Find(_ context.Context, _, tokenHash string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findErr != nil {
		return "", false, s.findErr
	}
	userID, ok := s.entries[tokenHash]
	return userID, ok, nil
}

func (s *recordingSessionStore) Revoke(_ context.Context, _, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, tokenHash)
	return nil
}

func (s *recordingSessionStore) remove(tokenHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, tokenHash)
}

func (s *recordingSessionStore) set(tokenHash, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[tokenHash] = userID
}

func (s *recordingSessionStore) contains(tokenHash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.entries[tokenHash]
	return ok
}

func newSessionConsistencyService(t *testing.T) (*Service, *recordingSessionStore) {
	t.Helper()
	repo, err := memory.New()
	if err != nil {
		t.Fatalf("memory repository: %v", err)
	}
	store := newRecordingSessionStore()
	svc := NewService(repo, store, events.NewRecordingPublisher(), []byte("session-consistency-signing-key"), time.Hour, 7*24*time.Hour)
	return svc, store
}

func sessionTenantContext() context.Context {
	return tenancy.WithActor(context.Background(), auth.Actor{TenantID: "upshs"})
}

func loginTeacherForSessionTest(t *testing.T, svc *Service) string {
	t.Helper()
	result, err := svc.LoginStart(sessionTenantContext(), "e.mensah@upshs.edu.gh", "password123")
	if err != nil || result.RefreshToken == "" {
		t.Fatalf("login: result=%+v err=%v", result, err)
	}
	return result.RefreshToken
}

func TestRefreshRequiresMatchingServerSession(t *testing.T) {
	svc, store := newSessionConsistencyService(t)
	refresh := loginTeacherForSessionTest(t, svc)
	hash := domain.HashToken(refresh)
	store.remove(hash)

	if _, _, _, _, err := svc.Refresh(context.Background(), refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("missing server session error=%v", err)
	}
	store.set(hash, "u-teacher")
	if _, _, _, _, err := svc.Refresh(context.Background(), refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("database family survived missing Redis session: %v", err)
	}
}

func TestRefreshRejectsSessionIdentityDrift(t *testing.T) {
	svc, store := newSessionConsistencyService(t)
	refresh := loginTeacherForSessionTest(t, svc)
	store.set(domain.HashToken(refresh), "different-user")

	if _, _, _, _, err := svc.Refresh(context.Background(), refresh); !errors.Is(err, domain.ErrExpiredToken) {
		t.Fatalf("identity drift error=%v", err)
	}
}

func TestRefreshRetriesAfterSessionStoreOutageWithoutConsumingToken(t *testing.T) {
	svc, store := newSessionConsistencyService(t)
	refresh := loginTeacherForSessionTest(t, svc)
	store.findErr = errors.New("redis unavailable")
	if _, _, _, _, err := svc.Refresh(context.Background(), refresh); !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("store outage error=%v", err)
	}
	store.findErr = nil
	if _, next, _, _, err := svc.Refresh(context.Background(), refresh); err != nil || next == "" {
		t.Fatalf("retry after store recovery: next=%q err=%v", next, err)
	}
}

func TestRefreshRotatesDatabaseAndServerSessionTogether(t *testing.T) {
	svc, store := newSessionConsistencyService(t)
	refresh := loginTeacherForSessionTest(t, svc)
	oldHash := domain.HashToken(refresh)
	access, next, user, expires, err := svc.Refresh(context.Background(), refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if access == "" || user.ID == "" || expires.IsZero() {
		t.Fatalf("incomplete refreshed session: user=%+v expires=%v", user, expires)
	}
	if store.contains(oldHash) || !store.contains(domain.HashToken(next)) {
		t.Fatalf("server session was not rotated with database token")
	}
}
