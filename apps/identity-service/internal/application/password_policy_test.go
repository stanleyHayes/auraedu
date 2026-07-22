package application

import (
	"strings"
	"testing"
)

func TestNewPasswordLengthPolicy(t *testing.T) {
	for _, test := range []struct {
		name     string
		password string
		valid    bool
	}{
		{name: "eleven characters", password: strings.Repeat("a", 11)},
		{name: "minimum", password: strings.Repeat("a", 12), valid: true},
		{name: "maximum", password: strings.Repeat("a", 256), valid: true},
		{name: "above maximum", password: strings.Repeat("a", 257)},
		{name: "unicode characters", password: strings.Repeat("🔐", 12), valid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validNewPassword(test.password); got != test.valid {
				t.Fatalf("validNewPassword()=%v want=%v", got, test.valid)
			}
		})
	}
}
