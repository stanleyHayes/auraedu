// Command api-gateway is the api-gateway service CLI.
package main

import (
	"log/slog"
	"os"

	servercmd "github.com/auraedu/api-gateway/cmd/server"
	"github.com/spf13/cobra"
)

const serviceName = "api-gateway"

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
	if err := root.Execute(); err != nil {
		slog.Default().Error("command failed", "err", err)
		os.Exit(1)
	}
}
