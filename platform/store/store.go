// Package store selects which persistence driver a service uses.
//
// AuraEDU ships two: PostgreSQL (platform/db) and MongoDB (platform/mongo).
// Neither is being removed. Services depend on their own ports, each driver has
// an adapter behind those ports, and this package decides which one is wired in
// at startup — so moving between them is configuration, not a rewrite, and
// moving back is the same.
package store

import (
	"fmt"
	"os"
	"strings"
)

type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMongo    Driver = "mongodb"
)

// EnvVar names the variable that selects the driver.
const EnvVar = "DATABASE_DRIVER"

// Selected reports the configured driver, defaulting to PostgreSQL so an
// existing deployment that sets nothing keeps its current behaviour.
//
// An unrecognised value is an error rather than a silent fallback: starting on
// the wrong database is worse than not starting.
func Selected() (Driver, error) {
	return Parse(os.Getenv(EnvVar))
}

func Parse(value string) (Driver, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(DriverPostgres), "postgresql", "pg":
		return DriverPostgres, nil
	case string(DriverMongo), "mongo":
		return DriverMongo, nil
	default:
		return "", fmt.Errorf("store: unknown %s %q: expected %q or %q",
			EnvVar, value, DriverPostgres, DriverMongo)
	}
}

// IsMongo is a readability helper for the wiring in each service's main.
func (d Driver) IsMongo() bool { return d == DriverMongo }
