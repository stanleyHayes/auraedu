package domain

import (
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	DomainPending  = "pending_dns"
	DomainVerified = "verified"
	DomainActive   = "active"
	DomainInactive = "inactive"
)

var domainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type CustomDomain struct {
	TenantCode        string     `json:"tenant_code"`
	Hostname          string     `json:"hostname"`
	Status            string     `json:"status"`
	TXTRecordName     string     `json:"txt_record_name"`
	VerificationToken string     `json:"verification_token,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	ActivatedAt       *time.Time `json:"activated_at,omitempty"`
	DeactivatedAt     *time.Time `json:"deactivated_at,omitempty"`
	ProviderReference string     `json:"provider_reference,omitempty"`
}

func NormalizeCustomDomain(value string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if len(host) < 4 || len(host) > 253 || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return "", ErrValidation
	}
	reserved := host == "auraedu.com" || strings.HasSuffix(host, ".auraedu.com") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".test")
	if reserved {
		return "", ErrValidation
	}
	for _, label := range strings.Split(host, ".") {
		if !domainLabel.MatchString(label) {
			return "", ErrValidation
		}
	}
	return host, nil
}

func DomainTXTRecord(hostname string) string { return "_auraedu." + hostname }
