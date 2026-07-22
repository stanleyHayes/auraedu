package domain

import (
	"encoding/base32"
	"testing"
	"time"
)

func TestValidateTOTPUsesRFC6238AndRejectsMalformedCodes(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	when := time.Unix(59, 0)

	counter, err := ValidateTOTP(secret, "287082", when)
	if err != nil {
		t.Fatalf("validate RFC 6238 code: %v", err)
	}
	if counter != 1 {
		t.Fatalf("counter = %d, want 1", counter)
	}
	for _, code := range []string{"", "12345", "1234567", "abcdef"} {
		if _, err := ValidateTOTP(secret, code, when); err == nil {
			t.Fatalf("malformed code %q was accepted", code)
		}
	}
}

func TestNewTOTPSecretHasAuthenticatorCompatibleEntropy(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("new secret: %v", err)
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil || len(raw) != 20 {
		t.Fatalf("secret decodes to %d bytes, err=%v", len(raw), err)
	}
}
