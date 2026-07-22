package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
	"github.com/google/uuid"
)

type deviceRepositoryStub struct {
	devices []*domain.DeviceToken
	invalid []string
}

func (r *deviceRepositoryStub) Upsert(context.Context, string, *domain.DeviceToken) (*domain.DeviceToken, error) {
	return nil, nil
}
func (r *deviceRepositoryStub) DeleteByDevice(context.Context, string, string, string) error {
	return nil
}
func (r *deviceRepositoryStub) ListActive(context.Context, string, string) ([]*domain.DeviceToken, error) {
	return r.devices, nil
}
func (r *deviceRepositoryStub) MarkInvalid(_ context.Context, _ string, token string) error {
	r.invalid = append(r.invalid, token)
	return nil
}

func TestExpoPushDeliversToAllActiveDevices(t *testing.T) {
	accessToken := uuid.NewString()
	repo := &deviceRepositoryStub{devices: []*domain.DeviceToken{
		{Token: expoDeviceCredential("ExponentPushToken")},
		{Token: expoDeviceCredential("ExpoPushToken")},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer "+accessToken {
			t.Errorf("Authorization = %q", got)
		}
		var body []expoPushRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body) != 2 || body[0].To != repo.devices[0].Token || body[1].To != repo.devices[1].Token {
			t.Fatalf("unexpected push payload: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"status":"ok","id":"one"},{"status":"ok","id":"two"}]}`))
	}))
	defer server.Close()
	notifier, err := NewExpoPushNotifier(
		repo,
		server.Client(),
		ExpoPushConfig{URL: server.URL, AccessToken: accessToken, AllowInsecure: true},
	)
	if err != nil {
		t.Fatalf("new notifier: %v", err)
	}
	message := domain.Message{ID: "message", TenantID: "tenant", RecipientID: "user", Channel: "push", Subject: "Attendance", Body: "Student marked absent", Metadata: map[string]any{}, CreatedAt: time.Now()}
	if err := notifier.Send(context.Background(), message); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestExpoPushRetiresDeviceNotRegistered(t *testing.T) {
	expiredDeviceCredential := expoDeviceCredential("ExponentPushToken")
	repo := &deviceRepositoryStub{devices: []*domain.DeviceToken{{Token: expiredDeviceCredential}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"status":"error","message":"Device is not registered","details":{"error":"DeviceNotRegistered"}}]}`))
	}))
	defer server.Close()
	notifier, err := NewExpoPushNotifier(repo, server.Client(), ExpoPushConfig{URL: server.URL, AllowInsecure: true})
	if err != nil {
		t.Fatalf("new notifier: %v", err)
	}
	err = notifier.Send(context.Background(), domain.Message{TenantID: "tenant", RecipientID: "user", Channel: "push", Subject: "Update", Body: "Body"})
	if err == nil {
		t.Fatal("provider ticket error must fail delivery")
	}
	if len(repo.invalid) != 1 || repo.invalid[0] != expiredDeviceCredential {
		t.Fatalf("invalid tokens = %#v", repo.invalid)
	}
}

func expoDeviceCredential(prefix string) string {
	return prefix + "[" + uuid.NewString() + "]"
}

func TestExpoPushRejectsInsecureProviderURL(t *testing.T) {
	repo := &deviceRepositoryStub{}
	if _, err := NewExpoPushNotifier(repo, nil, ExpoPushConfig{URL: "http://example.test"}); err == nil {
		t.Fatal("expected insecure URL rejection")
	}
}
