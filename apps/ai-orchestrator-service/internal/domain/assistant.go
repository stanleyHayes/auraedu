// Package domain defines the AI orchestrator's core entities and errors.
package domain

import (
	"errors"
	"time"
)

var (
	ErrValidation   = errors.New("assistant request validation failed")
	ErrConflict     = errors.New("assistant idempotency conflict")
	ErrUnavailable  = errors.New("approved knowledge is unavailable")
	ErrForbidden    = errors.New("assistant feature is unavailable")
	ErrNotFound     = errors.New("resource not found")
	ErrProhibited   = errors.New("AI action is not permitted by policy")
	ErrInvalidState = errors.New("AI action is not in the required state")
)

type Citation struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Version  int    `json:"version"`
}

type Response struct {
	TenantID          string     `json:"-"`
	SessionID         string     `json:"session_id"`
	MessageID         string     `json:"message_id"`
	Question          string     `json:"-"`
	Answer            string     `json:"answer"`
	Confidence        float64    `json:"confidence"`
	Citations         []Citation `json:"citations"`
	NeedsHuman        bool       `json:"needs_human"`
	EscalationMessage *string    `json:"escalation_message"`
	Locale            string     `json:"locale"`
	CreatedAt         time.Time  `json:"created_at"`
}

type KnowledgeResult struct {
	SourceID  string     `json:"source_id"`
	Title     string     `json:"title"`
	Passage   string     `json:"passage"`
	Locale    string     `json:"locale"`
	Version   int        `json:"version"`
	Score     float64    `json:"score"`
	ExpiresAt *time.Time `json:"expires_at"`
}
