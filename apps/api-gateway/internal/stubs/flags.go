// Package stubs provides local stand-ins for platform dependencies until they land.
package stubs

import "context"

type FeatureFlagClient struct {
	Defaults        map[string]bool
	TenantOverrides map[string]map[string]bool
}

func (c *FeatureFlagClient) IsEnabled(_ context.Context, tenantID, feature string) bool {
	if c.TenantOverrides != nil {
		if t, ok := c.TenantOverrides[tenantID]; ok {
			if v, ok := t[feature]; ok {
				return v
			}
		}
	}
	if c.Defaults != nil {
		if v, ok := c.Defaults[feature]; ok {
			return v
		}
	}
	return false
}
