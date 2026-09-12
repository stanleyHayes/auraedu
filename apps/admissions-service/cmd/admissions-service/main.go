package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	servercmd "github.com/auraedu/admissions-service/cmd/server"
	workercmd "github.com/auraedu/admissions-service/cmd/worker"
	"github.com/auraedu/admissions-service/internal/persistence"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{Use: "admissions-service", Version: version}
	root.AddCommand(&cobra.Command{Use: "server", RunE: func(*cobra.Command, []string) error { return servercmd.Run(version) }})
	root.AddCommand(&cobra.Command{Use: "worker", RunE: func(*cobra.Command, []string) error { return workercmd.Run() }})
	root.AddCommand(&cobra.Command{Use: "migrate", RunE: func(*cobra.Command, []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		database, e := persistence.Open(ctx)
		if database != nil {
			database.Close()
		}
		return e
	}})
	if e := root.Execute(); e != nil {
		slog.Error("command failed", "err", e)
		os.Exit(1)
	}
}
