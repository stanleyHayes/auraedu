package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCallbackRequestValidatesWindowTimezoneAndLocale(t *testing.T) {
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	callback, err := NewCallbackRequest("school-a", uuid.NewString(), now.Add(time.Hour), "Africa/Accra", "fr-GH", now)
	if err != nil {
		t.Fatal(err)
	}
	if callback.Status != CallbackRequested || callback.Locale != "fr-GH" || !callback.PreferredAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected callback: %+v", callback)
	}

	cases := []struct {
		name      string
		preferred time.Time
		timezone  string
		locale    string
	}{
		{name: "too soon", preferred: now.Add(14 * time.Minute), timezone: "Africa/Accra", locale: "en"},
		{name: "too far", preferred: now.Add(91 * 24 * time.Hour), timezone: "Africa/Accra", locale: "en"},
		{name: "invalid timezone", preferred: now.Add(time.Hour), timezone: "Accra", locale: "en"},
		{name: "unsupported locale", preferred: now.Add(time.Hour), timezone: "Africa/Accra", locale: "de"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewCallbackRequest("school-a", uuid.NewString(), tc.preferred, tc.timezone, tc.locale, now); !errors.Is(err, ErrValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}
