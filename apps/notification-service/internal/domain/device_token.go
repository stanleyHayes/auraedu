package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DeviceToken struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	Platform   string    `json:"platform"`
	Token      string    `json:"token"`
	Status     string    `json:"status"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NewDeviceToken(tenantID, userID, deviceID, platform, token string) (*DeviceToken, error) {
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	platform = strings.ToLower(strings.TrimSpace(platform))
	token = strings.TrimSpace(token)
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if userID == "" || deviceID == "" {
		return nil, fmt.Errorf("%w: user_id and device_id are required", ErrValidation)
	}
	if platform != "ios" && platform != "android" {
		return nil, fmt.Errorf("%w: platform must be ios or android", ErrValidation)
	}
	hasValidPrefix := strings.HasPrefix(token, "ExponentPushToken[") ||
		strings.HasPrefix(token, "ExpoPushToken[")
	if !hasValidPrefix || !strings.HasSuffix(token, "]") || len(token) > 255 {
		return nil, fmt.Errorf("%w: invalid Expo push token", ErrValidation)
	}
	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	return &DeviceToken{
		ID:         id.String(),
		TenantID:   tenantID,
		UserID:     userID,
		DeviceID:   deviceID,
		Platform:   platform,
		Token:      token,
		Status:     "active",
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}
