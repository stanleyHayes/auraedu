package db

import (
	"strings"
	"testing"
)

func TestMigrationUpSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      string
		wantError bool
	}{
		{
			name: "legacy forward-only migration",
			body: "CREATE TABLE users (id uuid);",
			want: "CREATE TABLE users (id uuid);",
		},
		{
			name: "goose migration excludes rollback",
			body: "-- +goose Up\nINSERT INTO roles VALUES ('admin');\n-- +goose Down\nDELETE FROM roles;",
			want: "INSERT INTO roles VALUES ('admin');",
		},
		{
			name:      "empty migration",
			body:      "  \n",
			wantError: true,
		},
		{
			name:      "empty up section",
			body:      "-- +goose Up\n\n-- +goose Down\nDELETE FROM roles;",
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrationUpSQL(test.body)
			if test.wantError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationUpSQL: %v", err)
			}
			if strings.TrimSpace(got) != test.want {
				t.Fatalf("unexpected forward SQL:\n%s", got)
			}
		})
	}
}
