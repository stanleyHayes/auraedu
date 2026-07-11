package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Claims is the JWT payload. Identity Service signs it on login; the gateway
// verifies it and turns it into an Actor + X-Actor-* headers for private services.
type Claims struct {
	Subject     string   `json:"sub"`
	TenantID    string   `json:"tenant_id,omitempty"`
	Role        string   `json:"role"`
	Permissions []string `json:"perms,omitempty"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
}

var (
	ErrInvalidToken = errors.New("auth: invalid token")
	ErrExpiredToken = errors.New("auth: token expired")
)

// HS256 header, pre-encoded (RawURL, no padding — per JWT spec).
const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// Sign issues an HS256 JWT for the claims using the shared signing key.
func Sign(c Claims, key []byte) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + sign(signingInput, key), nil
}

// Verify parses and validates an HS256 JWT (signature + expiry) and returns its claims.
func Verify(token string, key []byte, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(signingInput, key)), []byte(parts[2])) {
		return Claims{}, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if c.ExpiresAt != 0 && now.Unix() >= c.ExpiresAt {
		return Claims{}, ErrExpiredToken
	}
	return c, nil
}

// Actor maps verified claims to an Actor (the gateway uses this to set X-Actor-* headers).
func (c Claims) Actor() Actor {
	return Actor{
		UserID:        c.Subject,
		TenantID:      c.TenantID,
		Role:          c.Role,
		Permissions:   c.Permissions,
		PlatformAdmin: c.Role == RolePlatformSuperAdmin,
	}
}

func sign(input string, key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
