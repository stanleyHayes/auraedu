package testkit

import (
	"context"
	"fmt"
	"testing"

	"github.com/auraedu/platform/tenancy"
)

func LeakAssertion(ctx context.Context, t testing.TB, ownerTenant, otherTenant string, writeFn func() error, readFn func() (bool, error)) {
	t.Helper()

	if err := writeFn(); err != nil {
		t.Fatalf("write record for %s: %v", ownerTenant, err)
	}

	visible, err := readFn()
	if err != nil {
		t.Fatalf("read record as %s: %v", otherTenant, err)
	}
	if visible {
		t.Fatalf("CROSS-TENANT LEAK: %s could read %s record", otherTenant, ownerTenant)
	}
}

func ScopedExec(ctx context.Context, tenantID string, fn func(context.Context) error) func() error {
	return func() error {
		return fn(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID}))
	}
}

func ScopedRead(ctx context.Context, tenantID string, fn func(context.Context) (bool, error)) func() (bool, error) {
	return func() (bool, error) {
		return fn(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID}))
	}
}

func AssertNoLeak(ctx context.Context, t *testing.T, tenants []string, insert func(context.Context, string) error, exists func(context.Context, string) (bool, error)) {
	for _, owner := range tenants {
		owner := owner
		t.Run(fmt.Sprintf("owner=%s", owner), func(t *testing.T) {
			if err := insert(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: owner}), owner); err != nil {
				t.Fatalf("insert as %s: %v", owner, err)
			}
			for _, other := range tenants {
				if owner == other {
					continue
				}
				other := other
				t.Run(fmt.Sprintf("other=%s", other), func(t *testing.T) {
					visible, err := exists(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: other}), owner)
					if err != nil {
						t.Fatalf("exists as %s: %v", other, err)
					}
					if visible {
						t.Fatalf("cross-tenant leak: %s saw %s data", other, owner)
					}
				})
			}
		})
	}
}
