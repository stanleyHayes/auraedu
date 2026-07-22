package unit

import (
	"context"
	"testing"

	"github.com/auraedu/notification-service/internal/application"
	"github.com/auraedu/notification-service/internal/domain"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type fakeDeviceRepo struct {
	device      *domain.DeviceToken
	devices     []*domain.DeviceToken
	deletedUser string
}

func (r *fakeDeviceRepo) Upsert(_ context.Context, _ string, d *domain.DeviceToken) (*domain.DeviceToken, error) {
	r.device = d
	return d, nil
}
func (r *fakeDeviceRepo) DeleteByDevice(_ context.Context, _, user, _ string) error {
	r.deletedUser = user
	return nil
}
func (r *fakeDeviceRepo) ListActive(context.Context, string, string) ([]*domain.DeviceToken, error) {
	return r.devices, nil
}
func (r *fakeDeviceRepo) MarkInvalid(context.Context, string, string) error { return nil }

func TestDeviceRegistrationBindsAuthenticatedActor(t *testing.T) {
	tenant, user := "11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	repo := &fakeDeviceRepo{}
	g := flags.NewStaticSnapshot()
	g.Set(tenant, application.FeatureNotifications, true)
	svc := application.NewService(nil, nil, nil, application.WithFeatureGate(g), application.WithDeviceTokenRepository(repo))
	actor := auth.Actor{UserID: user, TenantID: tenant, Role: "student", Permissions: []string{application.PermRead}}
	ctx := tenancy.WithContext(context.Background(), tenancy.TenantContext{TenantID: tenant})
	device, err := svc.RegisterDeviceToken(ctx, actor, "phone-1", "android", "ExponentPushToken[test-token]")
	if err != nil {
		t.Fatal(err)
	}
	if device.UserID != user || device.TenantID != tenant {
		t.Fatalf("device not actor-bound: %+v", device)
	}
	if err := svc.UnregisterDeviceToken(ctx, actor, "phone-1"); err != nil {
		t.Fatal(err)
	}
	if repo.deletedUser != user {
		t.Fatalf("unregister user=%q", repo.deletedUser)
	}
}
