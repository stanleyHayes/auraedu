// Package jwt is a local Identity Service JWT signer.
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Subject      string   `json:"sub"`
	TenantID     string   `json:"tenant_id,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	Role         string   `json:"role"`
	Permissions  []string `json:"permissions,omitempty"`
	FeaturesHash string   `json:"features_hash,omitempty"`
	TokenType    string   `json:"typ,omitempty"`
	IssuedAt     int64    `json:"iat"`
	ExpiresAt    int64    `json:"exp"`
}

var (
	ErrInvalidToken = errors.New("jwt: invalid token")
	ErrExpiredToken = errors.New("jwt: token expired")
)

var header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

func Sign(c Claims, key []byte) (string, error) {
	c.TokenType = "access"
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	return signingInput + "." + sign(signingInput, key), nil
}

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

func sign(input string, key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
