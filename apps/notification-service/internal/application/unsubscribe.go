package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/auraedu/notification-service/internal/domain"
)

const unsubscribeValidity = 180 * 24 * time.Hour

type UnsubscribeManager struct {
	key          []byte
	publicAppURL string
	now          func() time.Time
}

type unsubscribeClaims struct {
	Version     int    `json:"v"`
	TenantID    string `json:"t"`
	AddressHash string `json:"h"`
	ExpiresAt   int64  `json:"e"`
}

func NewUnsubscribeManager(key, publicAppURL string) (*UnsubscribeManager, error) {
	key = strings.TrimSpace(key)
	if len(key) < 32 {
		return nil, fmt.Errorf("notifications: unsubscribe signing key must contain at least 32 characters")
	}
	if err := ValidatePublicAppURL(publicAppURL, false); err != nil {
		return nil, err
	}
	return &UnsubscribeManager{key: []byte(key), publicAppURL: strings.TrimRight(publicAppURL, "/"), now: time.Now}, nil
}

func (m *UnsubscribeManager) Link(tenantID, addressHash string) (string, error) {
	claims := unsubscribeClaims{
		Version: 1, TenantID: strings.TrimSpace(tenantID), AddressHash: strings.TrimSpace(addressHash),
		ExpiresAt: m.now().UTC().Add(unsubscribeValidity).Unix(),
	}
	if claims.TenantID == "" || !validAddressHash(claims.AddressHash) {
		return "", domain.ErrValidation
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("notifications: encode unsubscribe claims: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	token := encoded + "." + base64.RawURLEncoding.EncodeToString(m.sign(encoded))
	return m.publicAppURL + "/unsubscribe#token=" + token, nil
}

func (m *UnsubscribeManager) Verify(token string) (string, string, error) {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 1024 {
		return "", "", domain.ErrValidation
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, m.sign(parts[0])) {
		return "", "", domain.ErrValidation
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", domain.ErrValidation
	}
	var claims unsubscribeClaims
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Version != 1 ||
		strings.TrimSpace(claims.TenantID) == "" || !validAddressHash(claims.AddressHash) ||
		claims.ExpiresAt < m.now().UTC().Unix() || claims.ExpiresAt > m.now().UTC().Add(unsubscribeValidity+time.Hour).Unix() {
		return "", "", domain.ErrValidation
	}
	return claims.TenantID, claims.AddressHash, nil
}

func (m *UnsubscribeManager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func validAddressHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
