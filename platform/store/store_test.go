package store

import (
	"testing"
)

func TestDriverDefaultsToPostgresAndRejectsUnknownValues(t *testing.T) {
	// An existing deployment sets nothing and must keep PostgreSQL.
	for _, value := range []string{"", "  ", "postgres", "PostgreSQL", "pg"} {
		got, err := Parse(value)
		if err != nil || got != DriverPostgres {
			t.Fatalf("Parse(%q) = %q, %v; want postgres", value, got, err)
		}
	}
	for _, value := range []string{"mongodb", "Mongo", " MONGODB "} {
		got, err := Parse(value)
		if err != nil || got != DriverMongo {
			t.Fatalf("Parse(%q) = %q, %v; want mongodb", value, got, err)
		}
	}
	// Starting against the wrong database is worse than refusing to start.
	for _, value := range []string{"mysql", "sqlite", "mong", "postgre"} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) accepted an unknown driver", value)
		}
	}
}

func TestSelectedReadsTheEnvironment(t *testing.T) {
	t.Setenv(EnvVar, "mongodb")
	got, err := Selected()
	if err != nil || !got.IsMongo() {
		t.Fatalf("Selected() = %q, %v; want mongodb", got, err)
	}
	t.Setenv(EnvVar, "")
	got, err = Selected()
	if err != nil || got != DriverPostgres {
		t.Fatalf("Selected() with no value = %q, %v; want postgres", got, err)
	}
}
