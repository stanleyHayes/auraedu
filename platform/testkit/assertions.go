// Package testkit provides shared test helpers for the AuraEDU platform,
// including Postgres containers, JWT signers and tenant isolation assertions.
package testkit

import (
	"context"
	"fmt"
	"testing"

	"github.com/auraedu/platform/tenancy"
)

// LeakAssertion verifies that a record written for ownerTenant cannot be read
// by otherTenant.
func LeakAssertion(_ context.Context, tb testing.TB, ownerTenant, otherTenant string, writeFn func() error, readFn func() (bool, error)) {
	tb.Helper()

	if err := writeFn(); err != nil {
		tb.Fatalf("write record for %s: %v", ownerTenant, err)
	}

	visible, err := readFn()
	if err != nil {
		tb.Fatalf("read record as %s: %v", otherTenant, err)
	}
	if visible {
		tb.Fatalf("CROSS-TENANT LEAK: %s could read %s record", otherTenant, ownerTenant)
	}
}

// ScopedExec returns a closure that runs fn under the given tenant context.
func ScopedExec(ctx context.Context, tenantID string, fn func(context.Context) error) func() error {
	return func() error {
		return fn(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID}))
	}
}

// ScopedRead returns a closure that runs fn under the given tenant context.
func ScopedRead(ctx context.Context, tenantID string, fn func(context.Context) (bool, error)) func() (bool, error) {
	return func() (bool, error) {
		return fn(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: tenantID}))
	}
}

// AssertNoLeak checks that no tenant in tenants can read another tenant's data.
func AssertNoLeak(
	ctx context.Context,
	t *testing.T,
	tenants []string,
	insert func(context.Context, string) error,
	exists func(context.Context, string) (bool, error),
) {
	t.Helper()
	for _, owner := range tenants {
		t.Run(fmt.Sprintf("owner=%s", owner), func(t *testing.T) {
			if err := insert(tenancy.WithContext(ctx, tenancy.TenantContext{TenantID: owner}), owner); err != nil {
				t.Fatalf("insert as %s: %v", owner, err)
			}
			for _, other := range tenants {
				if owner == other {
					continue
				}
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
