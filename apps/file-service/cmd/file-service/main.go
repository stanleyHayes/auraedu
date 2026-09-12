// Command file-service is the file-service service CLI.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	servercmd "github.com/auraedu/file-service/cmd/server"
	workercmd "github.com/auraedu/file-service/cmd/worker"
	"github.com/auraedu/file-service/internal/persistence"
	"github.com/spf13/cobra"
)

const serviceName = "file-service"

var version = ""

func main() {
	root := &cobra.Command{
		Use:     serviceName,
		Short:   serviceName + " service CLI",
		Version: version,
	}
	root.AddCommand(&cobra.Command{
		Use:   "server",
		Short: "Run the " + serviceName + " HTTP server",
		RunE: func(_ *cobra.Command, _ []string) error {
			return servercmd.Run(version)
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "worker",
		Short: "Dispatch durable file lifecycle events and storage cleanup",
		RunE: func(_ *cobra.Command, _ []string) error {
			return workercmd.Run(version)
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
	database, err := persistence.Open(ctx)
	if err != nil {
		return err
	}
	database.Close()
	return nil
}
