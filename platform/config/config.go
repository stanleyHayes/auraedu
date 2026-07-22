// Package config loads and validates service configuration from the environment.
// Secrets come only from env / Render env groups — never from committed files
// (agent_plan §2 rule 10).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Getenv returns the env var or a fallback default.
func Getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// ServiceURL normalizes service discovery values into an HTTP base URL.
// Render's hostport references are scheme-less (for example tenant-service:10000).
func ServiceURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" || strings.Contains(value, "://") {
		return value
	}
	return "http://" + value
}

// MustGetenv returns the env var or errors if unset — use for required secrets.
func MustGetenv(key string) (string, error) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v, nil
	}
	return "", fmt.Errorf("required env var %q is not set", key)
}

// RequireProductionEnv validates variables that may remain optional for local
// development but are mandatory in a production process.
func RequireProductionEnv(keys ...string) error {
	if !strings.EqualFold(strings.TrimSpace(Getenv("ENVIRONMENT", "development")), "production") {
		return nil
	}
	for _, key := range keys {
		if _, err := MustGetenv(key); err != nil {
			return err
		}
	}
	return nil
}

// Port resolves the HTTP port. Render injects PORT for web services; default 8080.
func Port(fallback int) int {
	if v, ok := os.LookupEnv("PORT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
