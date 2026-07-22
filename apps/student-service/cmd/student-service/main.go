// Command student-service is the student-service service CLI.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/auraedu/platform/config"
	"github.com/auraedu/platform/db"
	servercmd "github.com/auraedu/student-service/cmd/server"
	workercmd "github.com/auraedu/student-service/cmd/worker"
	"github.com/spf13/cobra"
)

const serviceName = "student-service"

var version = ""

func main() {
	root := &cobra.Command{
		Use:     serviceName,
		Short:   serviceName + " service CLI",
		Version: version,
	}
	root.AddCommand(&cobra.Command{
		Use: "worker", Short: "Run the " + serviceName + " outbox worker",
		RunE: func(_ *cobra.Command, _ []string) error { return workercmd.Run(version) },
	})
	root.AddCommand(&cobra.Command{
		Use:   "server",
		Short: "Run the " + serviceName + " HTTP server",
		RunE: func(_ *cobra.Command, _ []string) error {
			return servercmd.Run(version)
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return runMigrate(ctx)
		},
	})
	if err := root.Execute(); err != nil {
		slog.Default().Error("command failed", "err", err)
		os.Exit(1)
	}
}

func runMigrate(ctx context.Context) error {
	dsn := config.Getenv("DATABASE_URL", "")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}
	database, err := db.Open(ctx, db.Config{DSN: dsn, Migrations: "migrations"})
	if err != nil {
		return err
	}
	database.Close()
	return nil
}
