package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	servercmd "github.com/auraedu/knowledge-service/cmd/server"
	workercmd "github.com/auraedu/knowledge-service/cmd/worker"
	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{Use: "knowledge-service", Version: version}
	root.AddCommand(&cobra.Command{Use: "server", RunE: func(*cobra.Command, []string) error { return servercmd.Run(version) }})
	root.AddCommand(&cobra.Command{Use: "worker", RunE: func(*cobra.Command, []string) error { return workercmd.Run(version) }})
	root.AddCommand(&cobra.Command{Use: "migrate", RunE: func(*cobra.Command, []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		dsn := config.Getenv("DATABASE_URL", "")
		if dsn == "" {
			return fmt.Errorf("DATABASE_URL not set")
		}
		database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: config.Getenv("MIGRATIONS_PATH", "migrations")})
		if database != nil {
			database.Close()
		}
		return err
	}})
	if err := root.Execute(); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
