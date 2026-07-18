package unit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/auraedu/fees-service/internal/application"
	"github.com/auraedu/fees-service/internal/domain"
	"github.com/auraedu/fees-service/internal/ports"
	"github.com/auraedu/platform/auth"
	"github.com/auraedu/platform/flags"
	"github.com/auraedu/platform/tenancy"
)

type fakeFeeStructureRepo struct{}

func (fakeFeeStructureRepo) Create(context.Context, string, *domain.FeeStructure) error { return nil }
func (fakeFeeStructureRepo) GetByID(context.Context, string, string) (*domain.FeeStructure, error) {
	return nil, domain.ErrNotFound
}
func (fakeFeeStructureRepo) List(context.Context, string, ports.FeeStructureFilter) ([]*domain.FeeStructure, string, error) {
	return nil, "", nil
}
func (fakeFeeStructureRepo) Update(context.Context, string, *domain.FeeStructure) error { return nil }
func (fakeFeeStructureRepo) Delete(context.Context, string, string) error               { return nil }

type fakeInvoiceRepo struct{}

func (fakeInvoiceRepo) Create(context.Context, string, *domain.Invoice) error { return nil }
func (fakeInvoiceRepo) GetByID(context.Context, string, string) (*domain.Invoice, error) {
	return nil, domain.ErrNotFound
}
func (fakeInvoiceRepo) List(context.Context, string, ports.InvoiceFilter) ([]*domain.Invoice, string, error) {
	return nil, "", nil
}
func (fakeInvoiceRepo) Update(context.Context, string, *domain.Invoice) error { return nil }
func (fakeInvoiceRepo) Delete(context.Context, string, string) error          { return nil }

// TestLiveFlagGateOverrideAndFallback wires the gate exactly like
// cmd/server: a live tenant-service client over a static registry snapshot.
// The static snapshot has "fees" disabled; the fake tenant-service enables
// it, then starts returning 5xx so the static fallback must rule again.
func TestLiveFlagGateOverrideAndFallback(t *testing.T) {
	static := flags.NewStaticSnapshot() // empty: every feature disabled

	var mu sync.Mutex
	fail := false
	actorHeader := ""
	tenantSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/features" || r.URL.Query().Get("tenant") != "upshs" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		actorHeader = r.Header.Get(auth.HeaderUserID)
		failing := fail
		mu.Unlock()
		if failing {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tenant_code":"upshs","features":[{"feature_key":"fees","is_enabled":true}]}`))
	}))
	defer tenantSvc.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	gate := flags.NewTenantServiceClient(tenantSvc.URL, flags.WarnOnceFallback(static, log))
	svc := application.NewService(fakeFeeStructureRepo{}, fakeInvoiceRepo{}, application.WithFeatureGate(gate))

	actor := auth.Actor{UserID: "u1", TenantID: "upshs", Role: "bursar", Permissions: []string{application.PermRead}}
	ctx := tenancy.WithContext(auth.WithActor(context.Background(), actor), tenancy.TenantContext{TenantID: "upshs"})

	// Live override: tenant-service enables "fees" although the static
	// snapshot has it disabled.
	if _, _, err := svc.ListFeeStructures(ctx, actor, ports.FeeStructureFilter{}); err != nil {
		t.Fatalf("expected live override to enable fees, got %v", err)
	}
	mu.Lock()
	gotActor := actorHeader
	mu.Unlock()
	if gotActor != "u1" {
		t.Fatalf("expected actor propagation (X-Actor-User=u1), got %q", gotActor)
	}

	// Tenant-service 5xx: the gate falls back to the static snapshot where
	// "fees" is disabled.
	mu.Lock()
	fail = true
	mu.Unlock()
	if _, _, err := svc.ListFeeStructures(ctx, actor, ports.FeeStructureFilter{}); !errors.Is(err, flags.ErrFeatureDisabled) {
		t.Fatalf("expected static fallback (feature disabled) on 5xx, got %v", err)
	}
}
